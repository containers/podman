package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containers/image/types"
	"github.com/containers/libpod/libpod"
	image2 "github.com/containers/libpod/libpod/image"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage/pkg/reexec"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/profile"
	"github.com/sirupsen/logrus"
)

const (
	defaultTestImage = "docker.io/library/alpine:latest"
	defaultRunCount  = 50
)

var helpMessage = `
-count int
	count of loop counter for test (default 50)
-image string
	image-name to be used for test (default "docker.io/library/alpine:latest")
-log string
	log level (info|debug|warn|error) (default "error")

`

func main() {

	ctx := context.Background()
	imageName := ""

	testImageName := flag.String("image", defaultTestImage, "image-name to be used for test")
	testRunCount := flag.Int("count", defaultRunCount, "count of loop counter for test")
	logLevel := flag.String("log", "error", "log level (info|debug|warn|error)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s \n", helpMessage)
	}

	flag.Parse()

	if reexec.Init() {
		return
	}

	switch strings.ToLower(*logLevel) {
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	default:
		logrus.Fatalf("invalid option : %s ", *logLevel)
	}

	opts := defaultRuntimeOptions()
	client, err := libpod.NewRuntime(opts...)
	if err != nil {
		logrus.Fatal(err)
	}
	defer client.Shutdown(false)

	// Print Runtime & System Information.
	err = printSystemInfo(client)
	if err != nil {
		logrus.Fatal(err)
	}

	imageRuntime := client.ImageRuntime()
	if imageRuntime == nil {
		logrus.Fatal("ImageRuntime is null")
	}

	fmt.Printf("preparing test environment...\n")
	//Prepare for test.
	testImage, err := imageRuntime.NewFromLocal(*testImageName)
	if err != nil {
		// Download the image from remote registry.
		writer := os.Stderr
		registryCreds := &types.DockerAuthConfig{
			Username: "",
			Password: "",
		}
		dockerRegistryOptions := image2.DockerRegistryOptions{
			DockerRegistryCreds:         registryCreds,
			DockerCertPath:              "",
			DockerInsecureSkipTLSVerify: types.OptionalBoolFalse,
		}
		fmt.Printf("image %s not found locally, fetching from remote registry..\n", *testImageName)

		testImage, err = client.ImageRuntime().New(ctx, *testImageName, "", "", writer, &dockerRegistryOptions, image2.SigningOptions{}, false)
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("image downloaded successfully\n\n")
	}

	names := testImage.Names()
	if len(names) > 0 {
		imageName = names[0]
	} else {
		imageName = testImage.ID()
	}

	idmappings, err := util.ParseIDMapping(nil, nil, "", "")
	if err != nil {
		logrus.Fatal(err)
	}
	config := &cc.CreateConfig{
		Tty:        true,
		Image:      imageName,
		ImageID:    testImage.ID(),
		IDMappings: idmappings,
		Command:    []string{"/bin/sh"},
		WorkDir:    "/",
		NetMode:    "bridge",
		Network:    "bridge",
	}

	// Enable CPU Profile
	defer profile.Start().Stop()

	data, err := runSingleThreadedStressTest(ctx, client, imageName, testImage.ID(), config, *testRunCount)
	if err != nil {
		logrus.Fatal(err)
	}

	data.printProfiledData((float64)(*testRunCount))
}

func defaultRuntimeOptions() []libpod.RuntimeOption {
	options := []libpod.RuntimeOption{}
	return options
	/*
		//TODO: Shall we test in clean environment?
		sOpts := storage.StoreOptions{
			GraphDriverName: "overlay",
			RunRoot:         "/var/run/containers/storage",
			GraphRoot:       "/var/lib/containers/storage",
		}

		storageOpts := libpod.WithStorageConfig(sOpts)
		options = append(options, storageOpts)
		return options
	*/
}

func printSystemInfo(client *libpod.Runtime) error {
	OCIRuntimeInfo, err := client.GetOCIRuntimeVersion()
	if err != nil {
		return err
	}

	connmanInfo, err := client.GetConmonVersion()
	if err != nil {
		return err
	}
	fmt.Printf("%s\n%s\n\n", OCIRuntimeInfo, connmanInfo)
	return nil
}

func runSingleThreadedStressTest(ctx context.Context, client *libpod.Runtime, imageName string, imageID string, config *cc.CreateConfig, testCount int) (*profileData, error) {
	data := new(profileData)
	fmt.Printf("Test Round: ")
	for i := 0; i < testCount; i++ {
		fmt.Printf("%d ", i)

		runtimeSpec, err := cc.CreateConfigToOCISpec(config)
		if err != nil {
			return nil, err
		}

		//Create Container
		networks := make([]string, 0)
		netmode := "bridge"
		createStartTime := time.Now()
		ctr, err := client.NewContainer(ctx,
			runtimeSpec,
			libpod.WithRootFSFromImage(imageID, imageName, false),
			libpod.WithNetNS([]ocicni.PortMapping{}, false, netmode, networks),
		)
		if err != nil {
			return nil, err
		}
		createTotalTime := time.Now().Sub(createStartTime)

		// Start container
		startStartTime := time.Now()
		err = ctr.Start(ctx)
		if err != nil {
			return nil, err
		}
		startTotalTime := time.Now().Sub(startStartTime)

		//Stop Container
		stopStartTime := time.Now()
		err = ctr.StopWithTimeout(2)
		if err != nil {
			return nil, err
		}
		stopTotalTime := time.Now().Sub(stopStartTime)

		//Delete Container
		deleteStartTime := time.Now()

		err = client.RemoveContainer(ctx, ctr, true)
		if err != nil {
			return nil, err
		}

		deleteTotalTime := time.Now().Sub(deleteStartTime)

		data.updateProfileData(createTotalTime, startTotalTime, stopTotalTime, deleteTotalTime)
	}
	return data, nil
}

type profileData struct {
	minCreate, minStart, minStop, minDel time.Duration
	avgCreate, avgStart, avgStop, avgDel time.Duration
	maxCreate, maxStart, maxStop, maxDel time.Duration
}

func (data *profileData) updateProfileData(create, start, stop, delete time.Duration) {
	if create < data.minCreate || data.minCreate == 0 {
		data.minCreate = create
	}
	if create > data.maxCreate || data.maxCreate == 0 {
		data.maxCreate = create
	}
	if start < data.minStart || data.minStart == 0 {
		data.minStart = start
	}
	if start > data.maxStart || data.maxStart == 0 {
		data.maxStart = start
	}
	if stop < data.minStop || data.minStop == 0 {
		data.minStop = stop
	}
	if stop > data.maxStop || data.maxStop == 0 {
		data.maxStop = stop
	}
	if delete < data.minDel || data.minDel == 0 {
		data.minDel = delete
	}
	if delete > data.maxDel || data.maxDel == 0 {
		data.maxDel = delete
	}

	data.avgCreate = data.avgCreate + create
	data.avgStart = data.avgStart + start
	data.avgStop = data.avgStop + stop
	data.avgDel = data.avgDel + delete
}

func (data *profileData) printProfiledData(testCount float64) {

	fmt.Printf("\nProfile data\n\n")
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "\tCreate\tStart\tStop\tDelete")
	fmt.Fprintf(w, "Min\t%.2fs\t%.2fs\t%.2fs\t%.2fs\n", data.minCreate.Seconds(), data.minStart.Seconds(), data.minStop.Seconds(), data.minDel.Seconds())
	fmt.Fprintf(w, "Avg\t%.2fs\t%.2fs\t%.2fs\t%.2fs\n", data.avgCreate.Seconds()/testCount, data.avgStart.Seconds()/testCount, data.avgStop.Seconds()/testCount, data.avgDel.Seconds()/testCount)
	fmt.Fprintf(w, "Max\t%.2fs\t%.2fs\t%.2fs\t%.2fs\n", data.maxCreate.Seconds(), data.maxStart.Seconds(), data.maxStop.Seconds(), data.maxDel.Seconds())
	fmt.Fprintf(w, "Total\t%.2fs\t%.2fs\t%.2fs\t%.2fs\n", data.avgCreate.Seconds(), data.avgStart.Seconds(), data.avgStop.Seconds(), data.avgDel.Seconds())
	fmt.Fprintln(w)
	w.Flush()
}
