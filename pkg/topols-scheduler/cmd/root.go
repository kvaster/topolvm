package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cybozu-go/well"
	"github.com/kvaster/topols"
	"github.com/kvaster/topols/scheduler"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var cfgFilePath string

const defaultListenAddr = ":8000"

// Config represents configuration parameters for topols-scheduler
type Config struct {
	// ListenAddr is listen address of topols-scheduler.
	ListenAddr string `json:"listen"`
	// Weights is a mapping between device-class names and their weights, default weight is 1.
	Weights map[string]float64 `json:"weights"`
}

var config = &Config{
	ListenAddr: defaultListenAddr,
}

var rootCmd = &cobra.Command{
	Use:     "topols-scheduler",
	Version: topols.Version,
	Short:   "a scheduler-extender for TopoLS",
	Long: `A scheduler-extender for TopoLS.

The extender implements filter and prioritize verbs.

The filter verb is "predicate" and served at "/predicate" via HTTP.
It filters out nodes that have less storage capacity than requested.
The requested capacity is read from "capacity.topols.kvaster.com/<device-class>"
resource value.

The prioritize verb is "prioritize" and served at "/prioritize" via HTTP.
For each device class request score is calculated with the following formula:

	(1 - requested / capacity)

Final (node) score is calculated in the following way:

    avg(device_class_score) * 10

Average is calculated with weights. By default each device class have weight of 1
and can be changed by config. 
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	err := well.LogConfig{}.Apply()
	if err != nil {
		return err
	}

	if len(cfgFilePath) != 0 {
		b, err := os.ReadFile(cfgFilePath)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(b, config)
		if err != nil {
			return err
		}
	}

	h, err := scheduler.NewHandler(config.Weights)
	if err != nil {
		return err
	}

	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    config.ListenAddr,
			Handler: h,
		},
	}

	err = serv.ListenAndServe()
	if err != nil {
		return err
	}
	err = well.Wait()

	if err != nil && !well.IsSignaled(err) {
		return err
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFilePath, "config", "", "config file")
}
