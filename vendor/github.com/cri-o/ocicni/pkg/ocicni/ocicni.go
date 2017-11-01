package ocicni

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

type cniNetworkPlugin struct {
	loNetwork *cniNetwork

	sync.RWMutex
	defaultNetwork *cniNetwork

	nsenterPath        string
	pluginDir          string
	cniDirs            []string
	vendorCNIDirPrefix string

	monitorNetDirChan chan struct{}

	// The pod map provides synchronization for a given pod's network
	// operations.  Each pod's setup/teardown/status operations
	// are synchronized against each other, but network operations of other
	// pods can proceed in parallel.
	podsLock sync.Mutex
	pods     map[string]*podLock
}

type cniNetwork struct {
	name          string
	NetworkConfig *libcni.NetworkConfigList
	CNIConfig     libcni.CNI
}

var errMissingDefaultNetwork = errors.New("Missing CNI default network")

type podLock struct {
	// Count of in-flight operations for this pod; when this reaches zero
	// the lock can be removed from the pod map
	refcount uint

	// Lock to synchronize operations for this specific pod
	mu sync.Mutex
}

func buildFullPodName(podNetwork PodNetwork) string {
	return podNetwork.Namespace + "_" + podNetwork.Name
}

// Lock network operations for a specific pod.  If that pod is not yet in
// the pod map, it will be added.  The reference count for the pod will
// be increased.
func (plugin *cniNetworkPlugin) podLock(podNetwork PodNetwork) *sync.Mutex {
	plugin.podsLock.Lock()
	defer plugin.podsLock.Unlock()

	fullPodName := buildFullPodName(podNetwork)
	lock, ok := plugin.pods[fullPodName]
	if !ok {
		lock = &podLock{}
		plugin.pods[fullPodName] = lock
	}
	lock.refcount++
	return &lock.mu
}

// Unlock network operations for a specific pod.  The reference count for the
// pod will be decreased.  If the reference count reaches zero, the pod will be
// removed from the pod map.
func (plugin *cniNetworkPlugin) podUnlock(podNetwork PodNetwork) {
	plugin.podsLock.Lock()
	defer plugin.podsLock.Unlock()

	fullPodName := buildFullPodName(podNetwork)
	lock, ok := plugin.pods[fullPodName]
	if !ok {
		logrus.Warningf("Unbalanced pod lock unref for %s", fullPodName)
		return
	} else if lock.refcount == 0 {
		// This should never ever happen, but handle it anyway
		delete(plugin.pods, fullPodName)
		logrus.Errorf("Pod lock for %s still in map with zero refcount", fullPodName)
		return
	}
	lock.refcount--
	lock.mu.Unlock()
	if lock.refcount == 0 {
		delete(plugin.pods, fullPodName)
	}
}

func (plugin *cniNetworkPlugin) monitorNetDir() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Errorf("could not create new watcher %v", err)
		return
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				logrus.Debugf("CNI monitoring event %v", event)
				if event.Op&fsnotify.Create != fsnotify.Create &&
					event.Op&fsnotify.Write != fsnotify.Write {
					continue
				}

				if err = plugin.syncNetworkConfig(); err == nil {
					logrus.Infof("CNI asynchronous setting succeeded")
					continue
				}

				logrus.Errorf("CNI setting failed, continue monitoring: %v", err)

			case err := <-watcher.Errors:
				logrus.Errorf("CNI monitoring error %v", err)
				close(plugin.monitorNetDirChan)
				return
			}
		}
	}()

	if err = watcher.Add(plugin.pluginDir); err != nil {
		logrus.Error(err)
		return
	}

	<-plugin.monitorNetDirChan
}

// InitCNI takes the plugin directory and cni directories where the cni files should be searched for
// Returns a valid plugin object and any error
func InitCNI(pluginDir string, cniDirs ...string) (CNIPlugin, error) {
	plugin := probeNetworkPluginsWithVendorCNIDirPrefix(pluginDir, cniDirs, "")
	var err error
	plugin.nsenterPath, err = exec.LookPath("nsenter")
	if err != nil {
		return nil, err
	}

	// check if a default network exists, otherwise dump the CNI search and return a noop plugin
	_, err = getDefaultCNINetwork(plugin.pluginDir, plugin.cniDirs, plugin.vendorCNIDirPrefix)
	if err != nil {
		if err != errMissingDefaultNetwork {
			logrus.Warningf("Error in finding usable CNI plugin - %v", err)
			// create a noop plugin instead
			return &cniNoOp{}, nil
		}

		// We do not have a default network, we start the monitoring thread.
		go plugin.monitorNetDir()
	}

	return plugin, nil
}

func probeNetworkPluginsWithVendorCNIDirPrefix(pluginDir string, cniDirs []string, vendorCNIDirPrefix string) *cniNetworkPlugin {
	plugin := &cniNetworkPlugin{
		defaultNetwork:     nil,
		loNetwork:          getLoNetwork(cniDirs, vendorCNIDirPrefix),
		pluginDir:          pluginDir,
		cniDirs:            cniDirs,
		vendorCNIDirPrefix: vendorCNIDirPrefix,
		monitorNetDirChan:  make(chan struct{}),
		pods:               make(map[string]*podLock),
	}

	// sync NetworkConfig in best effort during probing.
	if err := plugin.syncNetworkConfig(); err != nil {
		logrus.Error(err)
	}
	return plugin
}

func getDefaultCNINetwork(pluginDir string, cniDirs []string, vendorCNIDirPrefix string) (*cniNetwork, error) {
	if pluginDir == "" {
		pluginDir = DefaultNetDir
	}
	if len(cniDirs) == 0 {
		cniDirs = []string{DefaultCNIDir}
	}

	files, err := libcni.ConfFiles(pluginDir, []string{".conf", ".conflist", ".json"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, errMissingDefaultNetwork
	}

	sort.Strings(files)
	for _, confFile := range files {
		var confList *libcni.NetworkConfigList
		if strings.HasSuffix(confFile, ".conflist") {
			confList, err = libcni.ConfListFromFile(confFile)
			if err != nil {
				logrus.Warningf("Error loading CNI config list file %s: %v", confFile, err)
				continue
			}
		} else {
			conf, err := libcni.ConfFromFile(confFile)
			if err != nil {
				logrus.Warningf("Error loading CNI config file %s: %v", confFile, err)
				continue
			}
			if conf.Network.Type == "" {
				logrus.Warningf("Error loading CNI config file %s: no 'type'; perhaps this is a .conflist?", confFile)
				continue
			}
			confList, err = libcni.ConfListFromConf(conf)
			if err != nil {
				logrus.Warningf("Error converting CNI config file %s to list: %v", confFile, err)
				continue
			}
		}
		if len(confList.Plugins) == 0 {
			logrus.Warningf("CNI config list %s has no networks, skipping", confFile)
			continue
		}
		logrus.Infof("CNI network %s (type=%v) is used from %s", confList.Name, confList.Plugins[0].Network.Type, confFile)
		// Search for vendor-specific plugins as well as default plugins in the CNI codebase.
		vendorDir := vendorCNIDir(vendorCNIDirPrefix, confList.Plugins[0].Network.Type)
		cninet := &libcni.CNIConfig{
			Path: append(cniDirs, vendorDir),
		}
		network := &cniNetwork{name: confList.Name, NetworkConfig: confList, CNIConfig: cninet}
		return network, nil
	}
	return nil, fmt.Errorf("No valid networks found in %s", pluginDir)
}

func vendorCNIDir(prefix, pluginType string) string {
	return fmt.Sprintf(VendorCNIDirTemplate, prefix, pluginType)
}

func getLoNetwork(cniDirs []string, vendorDirPrefix string) *cniNetwork {
	if len(cniDirs) == 0 {
		cniDirs = []string{DefaultCNIDir}
	}

	loConfig, err := libcni.ConfListFromBytes([]byte(`{
  "cniVersion": "0.2.0",
  "name": "cni-loopback",
  "plugins": [{
    "type": "loopback"
  }]
}`))
	if err != nil {
		// The hardcoded config above should always be valid and unit tests will
		// catch this
		panic(err)
	}
	vendorDir := vendorCNIDir(vendorDirPrefix, loConfig.Plugins[0].Network.Type)
	cninet := &libcni.CNIConfig{
		Path: append(cniDirs, vendorDir),
	}
	loNetwork := &cniNetwork{
		name:          "lo",
		NetworkConfig: loConfig,
		CNIConfig:     cninet,
	}

	return loNetwork
}

func (plugin *cniNetworkPlugin) syncNetworkConfig() error {
	network, err := getDefaultCNINetwork(plugin.pluginDir, plugin.cniDirs, plugin.vendorCNIDirPrefix)
	if err != nil {
		logrus.Errorf("error updating cni config: %s", err)
		return err
	}
	plugin.setDefaultNetwork(network)

	return nil
}

func (plugin *cniNetworkPlugin) getDefaultNetwork() *cniNetwork {
	plugin.RLock()
	defer plugin.RUnlock()
	return plugin.defaultNetwork
}

func (plugin *cniNetworkPlugin) setDefaultNetwork(n *cniNetwork) {
	plugin.Lock()
	defer plugin.Unlock()
	plugin.defaultNetwork = n
}

func (plugin *cniNetworkPlugin) checkInitialized() error {
	if plugin.getDefaultNetwork() == nil {
		return errors.New("cni config uninitialized")
	}
	return nil
}

func (plugin *cniNetworkPlugin) Name() string {
	return CNIPluginName
}

func (plugin *cniNetworkPlugin) SetUpPod(podNetwork PodNetwork) error {
	if err := plugin.checkInitialized(); err != nil {
		return err
	}

	plugin.podLock(podNetwork).Lock()
	defer plugin.podUnlock(podNetwork)

	_, err := plugin.loNetwork.addToNetwork(podNetwork)
	if err != nil {
		logrus.Errorf("Error while adding to cni lo network: %s", err)
		return err
	}

	_, err = plugin.getDefaultNetwork().addToNetwork(podNetwork)
	if err != nil {
		logrus.Errorf("Error while adding to cni network: %s", err)
		return err
	}

	return err
}

func (plugin *cniNetworkPlugin) TearDownPod(podNetwork PodNetwork) error {
	if err := plugin.checkInitialized(); err != nil {
		return err
	}

	plugin.podLock(podNetwork).Lock()
	defer plugin.podUnlock(podNetwork)

	return plugin.getDefaultNetwork().deleteFromNetwork(podNetwork)
}

// TODO: Use the addToNetwork function to obtain the IP of the Pod. That will assume idempotent ADD call to the plugin.
// Also fix the runtime's call to Status function to be done only in the case that the IP is lost, no need to do periodic calls
func (plugin *cniNetworkPlugin) GetPodNetworkStatus(podNetwork PodNetwork) (string, error) {
	plugin.podLock(podNetwork).Lock()
	defer plugin.podUnlock(podNetwork)

	ip, err := getContainerIP(plugin.nsenterPath, podNetwork.NetNS, DefaultInterfaceName, "-4")
	if err != nil {
		return "", err
	}

	return ip.String(), nil
}

func (network *cniNetwork) addToNetwork(podNetwork PodNetwork) (cnitypes.Result, error) {
	rt, err := buildCNIRuntimeConf(podNetwork)
	if err != nil {
		logrus.Errorf("Error adding network: %v", err)
		return nil, err
	}

	netconf, cninet := network.NetworkConfig, network.CNIConfig
	logrus.Infof("About to add CNI network %s (type=%v)", netconf.Name, netconf.Plugins[0].Network.Type)
	res, err := cninet.AddNetworkList(netconf, rt)
	if err != nil {
		logrus.Errorf("Error adding network: %v", err)
		return nil, err
	}

	return res, nil
}

func (network *cniNetwork) deleteFromNetwork(podNetwork PodNetwork) error {
	rt, err := buildCNIRuntimeConf(podNetwork)
	if err != nil {
		logrus.Errorf("Error deleting network: %v", err)
		return err
	}

	netconf, cninet := network.NetworkConfig, network.CNIConfig
	logrus.Infof("About to del CNI network %s (type=%v)", netconf.Name, netconf.Plugins[0].Network.Type)
	err = cninet.DelNetworkList(netconf, rt)
	if err != nil {
		logrus.Errorf("Error deleting network: %v", err)
		return err
	}
	return nil
}

func buildCNIRuntimeConf(podNetwork PodNetwork) (*libcni.RuntimeConf, error) {
	logrus.Infof("Got pod network %+v", podNetwork)

	rt := &libcni.RuntimeConf{
		ContainerID: podNetwork.ID,
		NetNS:       podNetwork.NetNS,
		IfName:      DefaultInterfaceName,
		Args: [][2]string{
			{"IgnoreUnknown", "1"},
			{"K8S_POD_NAMESPACE", podNetwork.Namespace},
			{"K8S_POD_NAME", podNetwork.Name},
			{"K8S_POD_INFRA_CONTAINER_ID", podNetwork.ID},
		},
	}

	if len(podNetwork.PortMappings) == 0 {
		return rt, nil
	}

	rt.CapabilityArgs = map[string]interface{}{
		"portMappings": podNetwork.PortMappings,
	}
	return rt, nil
}

func (plugin *cniNetworkPlugin) Status() error {
	return plugin.checkInitialized()
}
