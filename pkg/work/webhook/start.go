package webhook

import (
	"context"
	"crypto/tls"

	"k8s.io/apimachinery/pkg/runtime"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/ocm/pkg/work/webhook/common"
	webhookv1 "open-cluster-management.io/ocm/pkg/work/webhook/v1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workv1.Install(scheme))
}

func (c *Options) RunWebhookServer() error {
	logger := klog.LoggerWithName(klog.FromContext(context.Background()), "work webhook")
	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: ":8000",
		WebhookServer: webhook.NewServer(webhook.Options{
			TLSOpts: []func(config *tls.Config){
				func(config *tls.Config) {
					config.MinVersion = tls.VersionTLS12
				},
			},
			Port:    c.Port,
			CertDir: c.CertDir,
		}),
	})
	if err != nil {
		logger.Error(err, "unable to start manager")
		return err
	}

	// add healthz/readyz check handler
	if err := mgr.AddHealthzCheck("healthz-ping", healthz.Ping); err != nil {
		logger.Error(err, "unable to add healthz check handler")
		return err
	}

	if err := mgr.AddReadyzCheck("readyz-ping", healthz.Ping); err != nil {
		logger.Error(err, "unable to add readyz check handler")
		return err
	}

	common.ManifestValidator.WithLimit(c.ManifestLimit)

	if err = (&webhookv1.ManifestWorkWebhook{}).Init(mgr); err != nil {
		logger.Error(err, "unable to create ManagedCluster webhook")
		return err
	}

	logger.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		return err
	}
	return nil
}
