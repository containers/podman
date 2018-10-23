package main

import (
	"fmt"
	"github.com/checkpoint-restore/go-criu"
	"github.com/checkpoint-restore/go-criu/rpc"
	"github.com/golang/protobuf/proto"
	"os"
	"strconv"
)

// TestNfy struct
type TestNfy struct {
	criu.NoNotify
}

// PreDump test function
func (c TestNfy) PreDump() error {
	fmt.Printf("TEST PRE DUMP\n")
	return nil
}

func doDump(c *criu.Criu, pidS string, imgDir string, pre bool, prevImg string) error {
	fmt.Printf("Dumping\n")
	pid, _ := strconv.Atoi(pidS)
	img, err := os.Open(imgDir)
	if err != nil {
		return fmt.Errorf("can't open image dir (%s)", err)
	}
	defer img.Close()

	opts := rpc.CriuOpts{
		Pid:         proto.Int32(int32(pid)),
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		LogLevel:    proto.Int32(4),
		LogFile:     proto.String("dump.log"),
	}

	if prevImg != "" {
		opts.ParentImg = proto.String(prevImg)
		opts.TrackMem = proto.Bool(true)
	}

	if pre {
		err = c.PreDump(opts, TestNfy{})
	} else {
		err = c.Dump(opts, TestNfy{})
	}
	if err != nil {
		return fmt.Errorf("dump fail (%s)", err)
	}

	return nil
}

// Usage: test $act $pid $images_dir
func main() {
	c := criu.MakeCriu()
	// Read out CRIU version
	version, err := c.GetCriuVersion()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("CRIU version", version)
	// Check if version at least 3.2
	result, err := c.IsCriuAtLeast(30200)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if !result {
		fmt.Println("CRIU too old")
		os.Exit(1)
	}
	act := os.Args[1]
	switch act {
	case "dump":
		err := doDump(c, os.Args[2], os.Args[3], false, "")
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
	case "dump2":
		err := c.Prepare()
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}

		err = doDump(c, os.Args[2], os.Args[3]+"/pre", true, "")
		if err != nil {
			fmt.Printf("pre-dump failed")
			fmt.Print(err)
			os.Exit(1)
		}
		err = doDump(c, os.Args[2], os.Args[3], false, "./pre")
		if err != nil {
			fmt.Printf("dump failed")
			fmt.Print(err)
			os.Exit(1)
		}

		c.Cleanup()
	case "restore":
		fmt.Printf("Restoring\n")
		img, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Printf("can't open image dir")
			os.Exit(1)
		}
		defer img.Close()

		opts := rpc.CriuOpts{
			ImagesDirFd: proto.Int32(int32(img.Fd())),
			LogLevel:    proto.Int32(4),
			LogFile:     proto.String("restore.log"),
		}

		err = c.Restore(opts, nil)
		if err != nil {
			fmt.Printf("Error:")
			fmt.Print(err)
			fmt.Printf("\n")
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown action\n")
		os.Exit(1)
	}

	fmt.Printf("Success\n")
}
