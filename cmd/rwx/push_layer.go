package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/rwx-cloud/cli/internal/oci"
	"github.com/spf13/cobra"
)

var (
	pushOCILayerPresignedURL string
	pushOCILayerRegistry     string
	pushOCILayerRepository   string
)

var pushLayerCmd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		registryURL, err := url.Parse(pushOCILayerRegistry)
		if err != nil {
			return err
		}
		registryURL.Scheme = "https"

		presignedLayerURL, err := url.Parse(pushOCILayerPresignedURL)
		if err != nil {
			return err
		}

		credentials, err := oci.PingRegistry(*registryURL)
		if err != nil {
			return err
		}

		res, err := http.Get(presignedLayerURL.String())
		if err != nil {
			return fmt.Errorf("unable to download layer from presigned URL: %w", err)
		}
		defer res.Body.Close()

		c := oci.NewClient(*registryURL, pushOCILayerRepository, credentials)
		if err := c.UploadLayer(res.Body); err != nil {
			return fmt.Errorf("unable to upload layer to OCI registry: %w", err)
		}

		return nil
	},
	Short: "Push an OCI layer to a registry",
	Use:   "layer --url <presigned-layer-url> --registry <registry> --repository <repository>",
}

func init() {
	pushLayerCmd.Flags().StringVar(&pushOCILayerPresignedURL, "url", "", "the presigned URL to download the layer")
	pushLayerCmd.Flags().StringVar(&pushOCILayerRegistry, "registry", "", "the OCI registry to push the layer to")
	pushLayerCmd.Flags().StringVar(&pushOCILayerRepository, "repository", "", "the OCI repository to push the layer to")
	pushLayerCmd.MarkFlagRequired("url")
	pushLayerCmd.MarkFlagRequired("registry")
	pushLayerCmd.MarkFlagRequired("repository")
}
