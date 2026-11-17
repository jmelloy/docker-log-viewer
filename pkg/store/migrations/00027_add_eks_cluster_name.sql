-- +goose Up
-- name becomes a unique label; cluster_name is the EKS cluster name (empty = name).
-- Needed when two accounts have clusters with the same name (common with kubeconfig imports).
ALTER TABLE eks_clusters ADD COLUMN cluster_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE eks_clusters DROP COLUMN cluster_name;
