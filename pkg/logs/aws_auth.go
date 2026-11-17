package logs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os/exec"
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

// SSOLogin starts `aws sso login` for the profile. The CLI opens a browser
// where it can and always prints the verification URL/code. We return the
// output from the first few seconds so the UI can show that URL; the CLI
// keeps waiting for approval in the background (up to 5 minutes) and writes
// the token to ~/.aws/sso/cache, where LoadAWSConfig picks it up.
func SSOLogin(profile string) (string, error) {
	args := []string{"sso", "login"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	cmd := exec.CommandContext(ctx, "aws", args...)
	var out lockedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("aws sso login (is the AWS CLI installed?): %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); cancel() }()
	select {
	case err := <-done:
		if err != nil {
			return out.String(), fmt.Errorf("aws sso login: %w", err)
		}
		return out.String(), nil
	case <-time.After(5 * time.Second):
		return out.String(), nil
	}
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
