/*
Copyright © 2026 Fabrice Tereszkiewicz @ A/Z&T <fabrice@azt.ch>
*/
package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var filename string
var filepathAbs string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "share-file [filename]",
	Short: "Share a great file with the fabulous world!",
	Long: `Use this tool to temporarely share a great file with the fabulous outside world, with a simple download link, how beautiful !

It requires your machine to be accessible directly via the internet. You can configure the hostname and port on 
which to share the file in environment variables or via arguments. The file you share must be in the current directory.

By default the server automatically stops after the file has been downloaded.`,

  // here we go!
  Run: func(cmd *cobra.Command, args []string) { 

    if len(args) == 1 {
      // share this file
      filename = args[0]
      filepath, err := filepath.Abs("./"+filename)

      if err != nil {
        fmt.Println(err)
          os.Exit(1)
      }

      filepathAbs = filepath

      // check file exists
      if _, err := os.Stat(filepathAbs); errors.Is(err, os.ErrNotExist) {
        fmt.Println("File ", filename, " not found in the current folder.")
        os.Exit(1)
      }

      // get server configuration
      hostname := viper.GetString("server.hostname")
      port := viper.GetInt("server.port")

      if hostname == "" || port == 0 {
        cmd.Usage()
        os.Exit(1)
      }

      // need a string
      portStr := strconv.Itoa(port)

      fmt.Println("Starting server... serving ", filepath)
      fmt.Println("\nhttp://" + hostname +  ":" + portStr + "/share/" + filename + "\n")
      http.HandleFunc("/share/"+ filename, fileHandler)
      log.Fatal(http.ListenAndServe(":" + portStr, nil))

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


func fileHandler(w http.ResponseWriter, r *http.Request) {
  IPAddress := ReadUserIP(r)
  fmt.Println("One hit for " + filename + " from " + IPAddress)
  http.ServeFile(w, r, filepathAbs)
  fmt.Println("\nBye bye")
  os.Exit(1)
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
