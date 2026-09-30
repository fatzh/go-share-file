/*
Copyright © 2026 Fabrice Tereszkiewicz @ A/Z&T <fabrice@azt.ch>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	filename    string
	filepathAbs string
	app         *tview.Application
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "share-file [filename]",
	Short: "Share a great file with the fabulous world!",
	Long: `Use this tool to temporarely share a great file with the fabulous outside world, with a simple download link, how beautiful !

It requires your machine to be accessible directly via the internet. You can configure the hostname and port on 
which to share the file in environment variables or via arguments. The file you share must be in the current directory.
`,

	// here we go!
	Run: func(cmd *cobra.Command, args []string) {
		app = tview.NewApplication()

		// show the TUI
		mainView := tview.NewGrid()

		// 2 simple text view
		statusView := tview.NewTextView()
		infoView := tview.NewTextView()

		// main view layout
		mainView.SetBorders(true).SetRows(5, 0)
		mainView.
			AddItem(statusView, 0, 0, 1, 3, 1, 1, false).
			AddItem(infoView, 1, 0, 1, 3, 1, 1, true)

		if len(args) == 1 {
			// share this file
			filename = args[0]
			absolutePath, err := filepath.Abs(filename)

			// check file exists, not a folder
			if err == nil {
				var fileInfo os.FileInfo
				fileInfo, err = os.Stat(absolutePath)

				if err != nil {
					err = fmt.Errorf("%s not found", absolutePath)

				} else if !fileInfo.Mode().IsRegular() {
					err = fmt.Errorf("%s is not a regular file", absolutePath)
				}
			}

			if err != nil {
				displayedPath := filepathAbs
				if displayedPath == "" {
					displayedPath = filename
				}
				statusView.SetText(fmt.Sprintf(
					"Unable to share file\n\nFile: %s\nError: %v\nPress any key to quit.",
					displayedPath,
					err,
				))

				mainView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					app.Stop()
					return nil
				})

				app.SetRoot(mainView, true)

				if runErr := app.Run(); runErr != nil {
					fmt.Fprintf(os.Stderr, "TUI error: %v\n", runErr)
				}

				// end here...
				return
			}

			filepathAbs = absolutePath
			publicName := filepath.Base(filepathAbs)

			// get server configuration
			hostname := viper.GetString("server.hostname")
			port := viper.GetInt("server.port")

			if hostname == "" || port == 0 {
				cmd.Usage()
				os.Exit(1)
			}

			fileUrl := fmt.Sprintf(
				"http://%s:%d/share/%s",
				hostname,
				port,
				url.PathEscape(publicName), // handle special characters
			)
			statusView.SetText(fmt.Sprintf("File link:\n%s\n\n'c' to copy to clipboard - 'q' to quit", fileUrl))

			// q to quit and c to copy
			mainView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Rune() {
				case 'q':
					app.Stop()
					return nil
				case 'c':
					// copy to clipboard
					if _, err := osc52.New(fileUrl).WriteTo(os.Stderr); err != nil {
						statusView.SetText(fmt.Sprintf(
							"Could not copy URL: %v",
							err,
						))
					}
					fmt.Fprintf(infoView, "URL copied to clipboard!\n")
					infoView.ScrollToEnd()
				}
				return event
			})

			// starts server
			http.HandleFunc(
				"/share/"+publicName,
				fileHandler(app, infoView, publicName, filepathAbs),
			)
			server := &http.Server{
				Addr: fmt.Sprintf(":%d", port),
			}

			// the TUI
			app.SetRoot(mainView, true)

			// coroutine for the server
			go func() {
				err := server.ListenAndServe()
				if err != nil && errors.Is(err, http.ErrServerClosed) {
					app.QueueUpdateDraw(func() {
						statusView.SetText(fmt.Sprintf("Server error: %v", err))
					})
				}
			}()

			if err := app.Run(); err != nil {
				panic(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
			}

		} else {
			cmd.Usage()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Use Viper to handle environment variables/config
	cobra.OnInitialize(initConfig)
	rootCmd.Flags().String("server-hostname", "", "Hostname for the server (env: SHARE_FILE_SERVER_HOSTNAME)")
	viper.BindPFlag("server.hostname", rootCmd.Flags().Lookup("server-hostname"))

	rootCmd.Flags().Int("server-port", 0, "Port number for the server (env: SHARE_FILE_SERVER_PORT)")
	viper.BindPFlag("server.port", rootCmd.Flags().Lookup("server-port"))
}

func initConfig() {
	viper.SetEnvPrefix("SHARE_FILE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

func fileHandler(
	app *tview.Application,
	infoView *tview.TextView,
	publicName string,
	filepathAbs string,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		ipAddress := ReadUserIP(r)

		message := fmt.Sprintf(
			"One hit for %s from %s\n",
			publicName,
			ipAddress,
		)

		app.QueueUpdateDraw(func() {
			fmt.Fprintf(infoView, message)
			infoView.ScrollToEnd()
		})

		http.ServeFile(w, r, filepathAbs)
	}
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
