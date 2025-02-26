package main

import (
	"fmt"
	"os"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{Use: "quota-control"}
	var accessKeyCmd = &cobra.Command{
		Use:   "accesskey",
		Short: "Manage access keys",
	}

	var verifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify an access key",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				fmt.Println("Usage: verify <access_key>")
				os.Exit(1)
			}
			fmt.Println("Verifying access key...")
			var errs []error
			for i := len(proto.SupportedEncodings) - 1; i >= 0; i-- {
				encoding := proto.SupportedEncodings[i]
				v := encoding.Version()

				projectID, ecosystemID, err := encoding.Decode(args[0])
				if err != nil {
					errs = append(errs, fmt.Errorf("v%d: %v", v, err))
					continue
				}
				fmt.Printf("Access key decoded => version:%d, projectID:%d, ecosystemID:%d\n", v, projectID, ecosystemID)
				return
			}

			fmt.Println("Access key is invalid:")
			for _, err := range errs {
				fmt.Println("-", err)
			}
		},
	}

	accessKeyCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(accessKeyCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
