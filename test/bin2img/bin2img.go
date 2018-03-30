package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"runtime"

	"github.com/containers/image/storage"
	"github.com/containers/image/types"
	sstorage "github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	if reexec.Init() {
		return
	}

	app := cli.NewApp()
	app.Name = "bin2img"
	app.Usage = "barebones image builder"
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "turn on debug logging",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "graph root directory",
		},
		cli.StringFlag{
			Name:  "runroot",
			Usage: "run root directory",
		},
		cli.StringFlag{
			Name:  "storage-driver",
			Usage: "storage driver",
		},
		cli.StringSliceFlag{
			Name:  "storage-opt",
			Usage: "storage option",
		},
		cli.StringFlag{
			Name:  "image-name",
			Usage: "set image name",
			Value: "kubernetes/pause",
		},
		cli.StringFlag{
			Name:  "source-binary",
			Usage: "source binary",
			Value: "../../pause/pause",
		},
		cli.StringFlag{
			Name:  "image-binary",
			Usage: "image binary",
			Value: "/pause",
		},
	}

	app.Action = func(c *cli.Context) error {
		debug := c.GlobalBool("debug")
		rootDir := c.GlobalString("root")
		runrootDir := c.GlobalString("runroot")
		storageDriver := c.GlobalString("storage-driver")
		storageOptions := c.GlobalStringSlice("storage-opt")
		imageName := c.GlobalString("image-name")
		sourceBinary := c.GlobalString("source-binary")
		imageBinary := c.GlobalString("image-binary")

		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.ErrorLevel)
		}
		if rootDir == "" && runrootDir != "" {
			logrus.Errorf("must set --root and --runroot, or neither")
			os.Exit(1)
		}
		if rootDir != "" && runrootDir == "" {
			logrus.Errorf("must set --root and --runroot, or neither")
			os.Exit(1)
		}
		storeOptions := sstorage.DefaultStoreOptions
		if rootDir != "" && runrootDir != "" {
			storeOptions.GraphDriverName = storageDriver
			storeOptions.GraphDriverOptions = storageOptions
			storeOptions.GraphRoot = rootDir
			storeOptions.RunRoot = runrootDir
		}
		store, err := sstorage.GetStore(storeOptions)
		if err != nil {
			logrus.Errorf("error opening storage: %v", err)
			os.Exit(1)
		}
		defer func() {
			_, _ = store.Shutdown(false)
		}()

		layerBuffer := &bytes.Buffer{}
		binary, err := os.Open(sourceBinary)
		if err != nil {
			logrus.Errorf("error opening image binary: %v", err)
			os.Exit(1)
		}
		binInfo, err := binary.Stat()
		if err != nil {
			logrus.Errorf("error statting image binary: %v", err)
			os.Exit(1)
		}
		archive := tar.NewWriter(layerBuffer)
		err = archive.WriteHeader(&tar.Header{
			Name:     imageBinary,
			Size:     binInfo.Size(),
			Mode:     0555,
			ModTime:  binInfo.ModTime(),
			Typeflag: tar.TypeReg,
			Uname:    "root",
			Gname:    "root",
		})
		if err != nil {
			logrus.Errorf("error writing archive header: %v", err)
			os.Exit(1)
		}
		_, err = io.Copy(archive, binary)
		if err != nil {
			logrus.Errorf("error archiving image binary: %v", err)
			os.Exit(1)
		}
		archive.Close()
		binary.Close()
		layerInfo := types.BlobInfo{
			Digest: digest.Canonical.FromBytes(layerBuffer.Bytes()),
			Size:   int64(layerBuffer.Len()),
		}

		ref, err := storage.Transport.ParseStoreReference(store, imageName)
		if err != nil {
			logrus.Errorf("error parsing image name: %v", err)
			os.Exit(1)
		}
		img, err := ref.NewImageDestination(nil)
		if err != nil {
			logrus.Errorf("error preparing to write image: %v", err)
			os.Exit(1)
		}
		defer img.Close()
		layer, err := img.PutBlob(layerBuffer, layerInfo, false)
		if err != nil {
			logrus.Errorf("error preparing to write image: %v", err)
			os.Exit(1)
		}
		config := &v1.Image{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
			Config: v1.ImageConfig{
				User:       "root",
				Entrypoint: []string{imageBinary},
			},
			RootFS: v1.RootFS{
				Type: "layers",
				DiffIDs: []digest.Digest{
					layer.Digest,
				},
			},
		}
		cbytes, err := json.Marshal(config)
		if err != nil {
			logrus.Errorf("error encoding configuration: %v", err)
			os.Exit(1)
		}
		configInfo := types.BlobInfo{
			Digest: digest.Canonical.FromBytes(cbytes),
			Size:   int64(len(cbytes)),
		}
		configInfo, err = img.PutBlob(bytes.NewBuffer(cbytes), configInfo, false)
		if err != nil {
			logrus.Errorf("error saving configuration: %v", err)
			os.Exit(1)
		}
		manifest := &v1.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: v1.Descriptor{
				MediaType: v1.MediaTypeImageConfig,
				Digest:    configInfo.Digest,
				Size:      int64(len(cbytes)),
			},
			Layers: []v1.Descriptor{{
				MediaType: v1.MediaTypeImageLayer,
				Digest:    layer.Digest,
				Size:      layer.Size,
			}},
		}
		mbytes, err := json.Marshal(manifest)
		if err != nil {
			logrus.Errorf("error encoding manifest: %v", err)
			os.Exit(1)
		}
		err = img.PutManifest(mbytes)
		if err != nil {
			logrus.Errorf("error saving manifest: %v", err)
			os.Exit(1)
		}
		err = img.Commit()
		if err != nil {
			logrus.Errorf("error committing image: %v", err)
			os.Exit(1)
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
