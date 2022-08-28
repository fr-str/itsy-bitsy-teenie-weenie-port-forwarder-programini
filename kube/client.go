package kube

import (
	"context"
	"fmt"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/config"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	crd "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log    = zap.S()
	Client *ClientS
)

// ClientS ...
type ClientS struct {
	Config  *restclient.Config
	API     *kubernetes.Clientset
	CRD     crd.Client
	CTX     context.Context
	Metrics *metricsv.Clientset

	IngressOldVersion bool
}

func Connect(configName string) {
	var err error
	Client, err = newClient(findConfig(configName))
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
		return
	}
	discover()
}

func findConfig(configName string) (kConfig []byte) {
	for _, v := range config.Config.KUBECONFIG_FOLDERS {
		if v == "" {
			continue
		}
		files, err := os.ReadDir(v)
		if err != nil {
			log.Error(err)
		}
		for _, filed := range files {
			if filed.IsDir() {
				continue
			}
			if strings.HasPrefix(filed.Name(), configName) {
				kConfig, err = os.ReadFile(filepath.Join(v, filed.Name()))
				if err != nil {
					log.Error(err)
				}
				return
			}

		}

	}
	log.Errorf("Config not found in specified folders %v", config.Config.KUBECONFIG_FOLDERS)
	os.Exit(1)
	return
}

func newClient(config []byte) (client *ClientS, err error) {

	kube := new(ClientS)

	if len(config) == 0 {
		kube.Config, err = ctrl.GetConfig()

	} else {
		kube.Config, err = clientcmd.RESTConfigFromKubeConfig(config)
	}

	if err != nil {
		return nil, err
	}

	kube.Config.RateLimiter = nil
	kube.Config.QPS = 1000
	kube.Config.Burst = 2000

	kube.API, err = kubernetes.NewForConfig(kube.Config)
	if err != nil {
		return nil, err
	}

	kube.CRD, err = crd.New(kube.Config, crd.Options{})
	if err != nil {
		return nil, err
	}

	kube.Metrics, err = metricsv.NewForConfig(kube.Config)
	if err != nil {
		return nil, err
	}

	kube.CTX = context.TODO()

	return kube, nil
}
