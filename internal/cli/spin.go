package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/briandowns/spinner"
)

var nonTTYTickInterval = 30 * time.Second

func Spin(message string, tty bool, out io.Writer) func() {
	if tty {
		indicator := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(out))
		indicator.Suffix = " " + message
		indicator.Start()
		return indicator.Stop
	} else {
		ticker := time.NewTicker(nonTTYTickInterval)
		fmt.Fprintln(out, message)
		go func() {
			for range ticker.C {
				fmt.Fprintf(out, ".")
			}
		}()

		return func() {
			ticker.Stop()
			fmt.Fprintln(out)
		}
	}
}

// SpinUntilDone shows a spinner while working, then replaces it with a final message.
// Returns a function that should be called with the final message to stop the spinner.
func SpinUntilDone(message string, tty bool, out io.Writer) func(finalMsg string) {
	if tty {
		indicator := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(out))
		indicator.Suffix = " " + message
		indicator.Start()
		return func(finalMsg string) {
			indicator.FinalMSG = finalMsg + "\n"
			indicator.Stop()
		}
	} else {
		ticker := time.NewTicker(nonTTYTickInterval)
		fmt.Fprintln(out, message)
		go func() {
			for range ticker.C {
				fmt.Fprintf(out, ".")
			}
		}()

		return func(finalMsg string) {
			ticker.Stop()
			fmt.Fprintln(out)
			fmt.Fprintln(out, finalMsg)
		}
	}
}
