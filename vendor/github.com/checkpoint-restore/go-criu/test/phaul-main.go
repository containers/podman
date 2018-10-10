package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/checkpoint-restore/go-criu"
	"github.com/checkpoint-restore/go-criu/phaul"
	"github.com/checkpoint-restore/go-criu/rpc"
	"github.com/golang/protobuf/proto"
)

type testLocal struct {
	criu.NoNotify
	r *testRemote
}

type testRemote struct {
	srv *phaul.Server
}

/* Dir where test will put dump images */
const imagesDir = "image"

func prepareImages() error {
	err := os.Mkdir(imagesDir, 0700)
	if err != nil {
		return err
	}

	/* Work dir for PhaulClient */
	err = os.Mkdir(imagesDir+"/local", 0700)
	if err != nil {
		return err
	}

	/* Work dir for PhaulServer */
	err = os.Mkdir(imagesDir+"/remote", 0700)
	if err != nil {
		return err
	}

	/* Work dir for DumpCopyRestore */
	err = os.Mkdir(imagesDir+"/test", 0700)
	if err != nil {
		return err
	}

	return nil
}

func mergeImages(dumpDir, lastPreDumpDir string) error {
	idir, err := os.Open(dumpDir)
	if err != nil {
		return err
	}

	defer idir.Close()

	imgs, err := idir.Readdirnames(0)
	if err != nil {
		return err
	}

	for _, fname := range imgs {
		if !strings.HasSuffix(fname, ".img") {
			continue
		}

		fmt.Printf("\t%s -> %s/\n", fname, lastPreDumpDir)
		err = syscall.Link(dumpDir+"/"+fname, lastPreDumpDir+"/"+fname)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *testRemote) doRestore() error {
	lastSrvImagesDir := r.srv.LastImagesDir()
	/*
	 * In imagesDir we have images from dump, in the
	 * lastSrvImagesDir -- where server-side images
	 * (from page server, with pages and pagemaps) are.
	 * Need to put former into latter and restore from
	 * them.
	 */
	err := mergeImages(imagesDir+"/test", lastSrvImagesDir)
	if err != nil {
		return err
	}

	imgDir, err := os.Open(lastSrvImagesDir)
	if err != nil {
		return err
	}
	defer imgDir.Close()

	opts := rpc.CriuOpts{
		LogLevel:    proto.Int32(4),
		LogFile:     proto.String("restore.log"),
		ImagesDirFd: proto.Int32(int32(imgDir.Fd())),
	}

	cr := r.srv.GetCriu()
	fmt.Printf("Do restore\n")
	return cr.Restore(opts, nil)
}

func (l *testLocal) PostDump() error {
	return l.r.doRestore()
}

func (l *testLocal) DumpCopyRestore(cr *criu.Criu, cfg phaul.Config, lastClnImagesDir string) error {
	fmt.Printf("Final stage\n")

	imgDir, err := os.Open(imagesDir + "/test")
	if err != nil {
		return err
	}
	defer imgDir.Close()

	psi := rpc.CriuPageServerInfo{
		Fd: proto.Int32(int32(cfg.Memfd)),
	}

	opts := rpc.CriuOpts{
		Pid:         proto.Int32(int32(cfg.Pid)),
		LogLevel:    proto.Int32(4),
		LogFile:     proto.String("dump.log"),
		ImagesDirFd: proto.Int32(int32(imgDir.Fd())),
		TrackMem:    proto.Bool(true),
		ParentImg:   proto.String(lastClnImagesDir),
		Ps:          &psi,
	}

	fmt.Printf("Do dump\n")
	return cr.Dump(opts, l)
}

func main() {
	pid, _ := strconv.Atoi(os.Args[1])
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		fmt.Printf("Can't make socketpair: %v\n", err)
		os.Exit(1)
	}

	err = prepareImages()
	if err != nil {
		fmt.Printf("Can't prepare dirs for images: %v\n", err)
		os.Exit(1)
		return
	}

	fmt.Printf("Make server part (socket %d)\n", fds[1])
	srv, err := phaul.MakePhaulServer(phaul.Config{
		Pid:   pid,
		Memfd: fds[1],
		Wdir:  imagesDir + "/remote"})
	if err != nil {
		fmt.Printf("Unable to run a server: %v", err)
		os.Exit(1)
		return
	}

	r := &testRemote{srv}

	fmt.Printf("Make client part (socket %d)\n", fds[0])
	cln, err := phaul.MakePhaulClient(&testLocal{r: r}, srv,
		phaul.Config{
			Pid:   pid,
			Memfd: fds[0],
			Wdir:  imagesDir + "/local"})
	if err != nil {
		fmt.Printf("Unable to run a client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Migrate\n")
	err = cln.Migrate()
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SUCCESS!\n")
}
