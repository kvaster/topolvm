package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kvaster/topols"
	"github.com/kvaster/topols/internal/scheduler"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
)

var cfgFilePath string
var zapOpts zap.Options

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
		return subMain(cmd.Context())
	},
}

func subMain(parentCtx context.Context) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := log.FromContext(parentCtx)

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

	serv := &http.Server{
		Addr:        config.ListenAddr,
		Handler:     accessLogHandler(parentCtx, h),
		ReadTimeout: 30 * time.Second,
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop() // stop() should be called before wg.Wait() to stop the goroutine correctly.

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := serv.Shutdown(parentCtx); err != nil {
			logger.Error(err, "failed to shutdown gracefully")
		}
	}()

	err = serv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
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
