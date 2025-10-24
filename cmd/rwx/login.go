package main

import (
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/skratchdot/open-golang/open"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	DeviceName                    string
	OpenMintLoginAuthorizationUrl bool

	loginCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			// try to collect the device name if one is not provided
			if DeviceName == "" {
				prompt := promptui.Prompt{
					Label:   "Device Name",
					Default: defaultDeviceName(),
					Validate: func(s string) error {
						if s == "" {
							return errors.New("Device name must be provided")
						}

						return nil
					},
				}
				deviceName, err := prompt.Run()
				if err != nil {
					return err
				}

				DeviceName = deviceName
			}

			openUrl := open.Run
			if !OpenMintLoginAuthorizationUrl {
				openUrl = func(input string) error { return nil }
			}

			err := service.Login(
				cli.LoginConfig{
					DeviceName:         DeviceName,
					AccessTokenBackend: accessTokenBackend,
					OpenUrl:            openUrl,
					PollInterval:       1 * time.Second,
				},
			)
			if err != nil {
				return err
			}

			return nil

		},
		Short: "Authorize subsequent commands on this device with RWX Cloud",
		Use:   "login [flags]",
	}
)

func init() {
	loginCmd.Flags().StringVar(&DeviceName, "device-name", "", "the name of the device logging in (if unset, you will be prompted to enter interactively)")
	loginCmd.Flags().BoolVar(&OpenMintLoginAuthorizationUrl, "open", true, "whether the authorization URL should automatically be opened in your browser")
}

func defaultDeviceName() string {
	user, _ := user.Current()
	host, _ := os.Hostname()

	if user != nil && host != "" {
		return fmt.Sprintf("%v@%v", user.Username, host)
	} else if user != nil {
		return user.Username
	} else if host != "" {
		return host
	}

	return ""
}
