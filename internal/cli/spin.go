package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/briandowns/spinner"
)

func Spin(message string, tty bool, out io.Writer) func() {
	if tty {
		indicator := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(out))
		indicator.Suffix = " " + message
		indicator.Start()
		return indicator.Stop
	} else {
		ticker := time.NewTicker(1 * time.Second)
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
