package main

import (
	"fmt"
	"net/url"

	"github.com/rwx-cloud/cli/internal/oci"
	"github.com/spf13/cobra"
)

var (
	pushOCIManifestRegistry   string
	pushOCIManifestRepository string
	pushOCIManifestLayers     []string
)

var pushManifestCmd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		registryURL, err := url.Parse(pushOCIManifestRegistry)
		if err != nil {
			return err
		}
		registryURL.Scheme = "https"

		credentials, err := oci.PingRegistry(*registryURL)
		if err != nil {
			return err
		}

		c := oci.NewClient(*registryURL, pushOCIManifestRepository, credentials)
		if err := c.UploadManifest(pushOCIManifestLayers); err != nil {
			return fmt.Errorf("unable to upload manifest to OCI registry: %w", err)
		}

		return nil
	},
	Short: "Push an OCI manifest to a registry",
	Use:   "manifest --registry <registry> --repository <repository> [--layer '<diff-id>|<digest>|<size-in-bytes>']...",
}

func init() {
	pushManifestCmd.Flags().StringVar(&pushOCIManifestRegistry, "registry", "", "the OCI registry to push the layer to")
	pushManifestCmd.Flags().StringVar(&pushOCIManifestRepository, "repository", "", "the OCI repository to push the layer to")
	pushManifestCmd.Flags().StringArrayVar(&pushOCIManifestLayers, "layer", []string{}, "the OCI layer to include in the manifest in the format '<diff-id>|<digest>|<size-in-bytes>' (can be specified multiple times)")
	pushManifestCmd.MarkFlagRequired("registry")
	pushManifestCmd.MarkFlagRequired("repository")
	pushManifestCmd.MarkFlagRequired("layer")
}
