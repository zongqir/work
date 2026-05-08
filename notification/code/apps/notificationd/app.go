package notificationd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"work/notification/code/internal/bootstrap"
	"work/notification/code/internal/dao"
	"work/notification/code/internal/dispatch"
	"work/notification/code/internal/metrics"
	"work/notification/code/internal/publisher"
	"work/notification/code/internal/scheduler"
	"work/notification/code/pkg/notification/contract"
)

const defaultShutdownTimeout = 5 * time.Second

type Config struct {
	Dispatcher *bootstrap.Config
	Consumer   *bootstrap.ConsumerConfig
	Scheduler  *SchedulerConfig
	Metrics    *MetricsConfig
}

type SchedulerConfig struct {
	LoadAll        func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	WatermarkStore dao.AggregateWatermarkStore
	LogError       func(ctx context.Context, msg string, err error)
	PollInterval   time.Duration
	Now            func() time.Time
}

type MetricsConfig struct {
	Addr              string
	Handler           http.Handler
	ReadHeaderTimeout time.Duration
}

type App struct {
	dispatcher    *dispatchRuntime
	consumer      *bootstrap.ConsumerService
	scheduler     *scheduler.AggregateScheduler
	metricsServer *http.Server
}

func New(config Config) (*App, error) {
	if config.Dispatcher == nil && config.Consumer == nil && config.Scheduler == nil && config.Metrics == nil {
		return nil, fmt.Errorf("%w: at least one app component is required", contract.ErrInvalidRequest)
	}

	app := &App{}

	if config.Dispatcher != nil {
		runtime, err := newDispatchRuntime(*config.Dispatcher)
		if err != nil {
			return nil, err
		}
		app.dispatcher = runtime
	}

	if config.Consumer != nil {
		consumerService, err := bootstrap.NewConsumer(*config.Consumer)
		if err != nil {
			app.Close()
			return nil, err
		}
		app.consumer = consumerService
	}

	if config.Scheduler != nil {
		if app.dispatcher == nil {
			app.Close()
			return nil, fmt.Errorf("%w: dispatcher config is required when scheduler is enabled", contract.ErrInvalidRequest)
		}
		if config.Scheduler.LoadAll == nil {
			app.Close()
			return nil, fmt.Errorf("%w: scheduler load_all is required", contract.ErrInvalidRequest)
		}
		if config.Scheduler.WatermarkStore == nil {
			app.Close()
			return nil, fmt.Errorf("%w: scheduler watermark store is required", contract.ErrInvalidRequest)
		}

		app.scheduler = &scheduler.AggregateScheduler{
			LoadAll:        config.Scheduler.LoadAll,
			Sender:         app.dispatcher.dispatcher,
			WatermarkStore: config.Scheduler.WatermarkStore,
			LogError:       config.Scheduler.LogError,
			PollInterval:   config.Scheduler.PollInterval,
			Now:            config.Scheduler.Now,
		}
	}

	if config.Metrics != nil {
		if config.Metrics.Addr == "" {
			app.Close()
			return nil, fmt.Errorf("%w: metrics addr is required", contract.ErrInvalidRequest)
		}

		handler := config.Metrics.Handler
		if handler == nil {
			handler = metrics.Handler()
		}
		readHeaderTimeout := config.Metrics.ReadHeaderTimeout
		if readHeaderTimeout <= 0 {
			readHeaderTimeout = 5 * time.Second
		}

		app.metricsServer = &http.Server{
			Addr:              config.Metrics.Addr,
			Handler:           handler,
			ReadHeaderTimeout: readHeaderTimeout,
		}
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("%w: app is nil", contract.ErrInvalidRequest)
	}
	if a.dispatcher == nil && a.consumer == nil && a.scheduler == nil && a.metricsServer == nil {
		return fmt.Errorf("%w: no runnable component configured", contract.ErrInvalidRequest)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)

	if a.consumer != nil {
		go func() {
			if err := a.consumer.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("consumer run failed: %w", err)
			}
		}()
	}

	if a.scheduler != nil {
		go func() {
			if err := a.scheduler.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("scheduler run failed: %w", err)
			}
		}()
	}

	if a.metricsServer != nil {
		go func() {
			<-runCtx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
			defer shutdownCancel()
			_ = a.metricsServer.Shutdown(shutdownCtx)
		}()
		go func() {
			if err := a.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("metrics server failed: %w", err)
			}
		}()
	}

	select {
	case err := <-errCh:
		cancel()
		a.Close()
		return err
	case <-ctx.Done():
		cancel()
		a.Close()
		return ctx.Err()
	}
}

func (a *App) Close() {
	if a == nil {
		return
	}
	if a.consumer != nil {
		a.consumer.Close()
	}
	if a.dispatcher != nil {
		a.dispatcher.Close()
	}
	if a.metricsServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer shutdownCancel()
		_ = a.metricsServer.Shutdown(shutdownCtx)
	}
}

func (a *App) Dispatcher() *dispatch.Dispatcher {
	if a == nil || a.dispatcher == nil {
		return nil
	}
	return a.dispatcher.dispatcher
}

type dispatchRuntime struct {
	dispatcher *dispatch.Dispatcher
	publisher  *publisher.PulsarPublisher
	client     pulsar.Client
}

func newDispatchRuntime(config bootstrap.Config) (*dispatchRuntime, error) {
	if config.PulsarClientOptions.URL == "" {
		return nil, fmt.Errorf("%w: pulsar url is required", contract.ErrInvalidRequest)
	}
	if config.Topic == "" {
		return nil, fmt.Errorf("%w: topic is required", contract.ErrInvalidRequest)
	}
	if config.LoadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}

	client, err := pulsar.NewClient(config.PulsarClientOptions)
	if err != nil {
		return nil, err
	}

	messagePublisher, err := publisher.NewPulsarPublisher(client, config.Topic)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &dispatchRuntime{
		dispatcher: &dispatch.Dispatcher{
			Publisher:            messagePublisher,
			LoadAll:              config.LoadAll,
			LogError:             config.LogError,
			CacheTTL:             config.CacheTTL,
			CacheMaxStale:        config.CacheMaxStale,
			RealtimeExpireAfter:  config.RealtimeExpireAfter,
			AggregateExpireAfter: config.AggregateExpireAfter,
		},
		publisher: messagePublisher,
		client:    client,
	}, nil
}

func (r *dispatchRuntime) Close() {
	if r == nil {
		return
	}
	if r.publisher != nil {
		r.publisher.Close()
	}
	if r.client != nil {
		r.client.Close()
	}
}
