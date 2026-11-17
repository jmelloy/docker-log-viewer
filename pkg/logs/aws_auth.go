package logs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// LoadAWSConfig resolves credentials the same way the AWS CLI does: env vars,
// shared config/credentials files, SSO token cache (~/.aws/sso/cache), and
// instance/container roles. An empty profile uses AWS_PROFILE or "default".
// The SDK wraps credentials in aws.CredentialsCache, so they're cached in
// memory and refreshed before expiry; nothing is written to disk here.
func LoadAWSConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config (profile %q): %w", profile, err)
	}
	return cfg, nil
}

// AWSIdentity returns the caller ARN, which doubles as a credentials check.
func AWSIdentity(ctx context.Context, cfg aws.Config) (string, error) {
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Arn), nil
}

// AWS profile kinds, which decide how the user re-authenticates.
const (
	ProfileKindSSO   = "sso"   // IAM Identity Center: `aws sso login`
	ProfileKindOther = "other" // credentials file (e.g. written by an Okta tool), credential_process, env
)

// AWSProfileKind reports whether a profile signs in through AWS SSO.
func AWSProfileKind(ctx context.Context, profile string) string {
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
	}
	if profile == "" {
		profile = "default"
	}
	sc, err := config.LoadSharedConfigProfile(ctx, profile, func(o *config.LoadSharedConfigOptions) {
		if f := os.Getenv("AWS_CONFIG_FILE"); f != "" {
			o.ConfigFiles = []string{f}
		}
		if f := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); f != "" {
			o.CredentialsFiles = []string{f}
		}
	})
	if err == nil && (sc.SSOSessionName != "" || sc.SSOStartURL != "") {
		return ProfileKindSSO
	}
	return ProfileKindOther
}

// ReauthHint is the user-facing message for expired/invalid credentials.
func ReauthHint(ctx context.Context, profile string) string {
	shown := profile
	if shown == "" {
		shown = "default"
	}
	if AWSProfileKind(ctx, profile) == ProfileKindSSO {
		login := "aws sso login"
		if profile != "" {
			login += " --profile " + profile
		}
		return fmt.Sprintf("AWS SSO session expired. Run '%s' or click Re-authenticate.", login)
	}
	return fmt.Sprintf("AWS credentials for profile '%s' are expired or invalid. Sign in again with your Okta/AWS "+
		"login tool (it refreshes ~/.aws/credentials); the cluster reconnects automatically once they're valid.", shown)
}

// awsCredsModTime returns the newest modification time of the places
// credentials appear after a sign-in: the shared credentials file (Okta
// tools, `aws configure`) and the SSO token cache directory.
func awsCredsModTime() time.Time {
	var newest time.Time
	paths := []string{config.DefaultSharedCredentialsFilename()}
	if f := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); f != "" {
		paths = []string{f}
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".aws", "sso", "cache"))
	}
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

var ssoURLRe = regexp.MustCompile(`https://\S+`)

// SSOLogin starts `aws sso login` with the device-code flow and no browser:
// the CLI prints a verification URL (with the code pre-filled) that the UI
// opens in the user's own browser, where they complete the Okta/IdP sign-in.
// We don't automate that browser step. The CLI keeps polling in the
// background (up to 5 minutes) and writes the token to ~/.aws/sso/cache.
func SSOLogin(profile string) (verifyURL, output string, err error) {
	args := []string{"sso", "login", "--no-browser", "--use-device-code"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	cmd := exec.CommandContext(ctx, "aws", args...)
	var out lockedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		cancel()
		return "", "", fmt.Errorf("aws sso login (is the AWS CLI installed?): %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); cancel() }()

	// Wait for the URL to be printed (or the CLI to exit).
	deadline := time.After(10 * time.Second)
	for {
		select {
		case err := <-done:
			if err != nil {
				return "", out.String(), fmt.Errorf("aws sso login: %w", err)
			}
			return "", out.String(), nil // already signed in
		case <-deadline:
			return "", out.String(), nil
		case <-time.After(200 * time.Millisecond):
			if u := verificationURL(out.String()); u != "" && strings.Contains(u, "user_code=") {
				return u, out.String(), nil
			}
		}
	}
}

// verificationURL picks the URL with the code pre-filled, else the first one.
func verificationURL(output string) string {
	urls := ssoURLRe.FindAllString(output, -1)
	for _, u := range urls {
		if strings.Contains(u, "user_code=") {
			return u
		}
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

const (
	eksTokenPrefix = "k8s-aws-v1."
	// EKS tokens are valid for 15 minutes; refresh a bit earlier.
	eksTokenTTL = 14 * time.Minute
)

// EKSToken builds a bearer token for the EKS API server, equivalent to
// `aws eks get-token`: a presigned STS GetCallerIdentity URL bound to the
// cluster name via the x-k8s-aws-id header.
func EKSToken(ctx context.Context, cfg aws.Config, clusterName string) (string, error) {
	presigner := sts.NewPresignClient(sts.NewFromConfig(cfg))
	req, err := presigner.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.PresignOptions) {
		o.ClientOptions = append(o.ClientOptions, func(so *sts.Options) {
			so.APIOptions = append(so.APIOptions,
				smithyhttp.AddHeaderValue("x-k8s-aws-id", clusterName),
				smithyhttp.AddHeaderValue("X-Amz-Expires", "60"),
			)
		})
	})
	if err != nil {
		return "", fmt.Errorf("presign EKS token: %w", err)
	}
	return eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(req.URL)), nil
}

// eksTokenTransport injects a cached, auto-refreshed EKS bearer token.
type eksTokenTransport struct {
	base    http.RoundTripper
	cfg     aws.Config
	cluster string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func (t *eksTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if time.Now().After(t.expires) {
		token, err := EKSToken(req.Context(), t.cfg, t.cluster)
		if err != nil {
			t.mu.Unlock()
			return nil, err
		}
		t.token, t.expires = token, time.Now().Add(eksTokenTTL)
	}
	token := t.token
	t.mu.Unlock()

	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(req)
}
