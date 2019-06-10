package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
	"github.com/varlink/go/varlink"
	"os"
	"strings"
)

var bold = color.New(color.Bold)
var errorBoldRed string
var bridge string

func ErrPrintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s ", errorBoldRed)
	fmt.Fprintf(os.Stderr, format, a...)
}

func print_usage(set *flag.FlagSet, arg_help string) {
	if set == nil {
		fmt.Fprintf(os.Stderr, "Usage: %s [GLOBAL OPTIONS] COMMAND ...\n", os.Args[0])
	} else {
		fmt.Fprintf(os.Stderr, "Usage: %s [GLOBAL OPTIONS] %s [OPTIONS] %s\n", os.Args[0], set.Name(), arg_help)
	}

	fmt.Fprintln(os.Stderr, "\nGlobal Options:")
	flag.PrintDefaults()

	if set == nil {
		fmt.Fprintln(os.Stderr, "\nCommands:")
		fmt.Fprintln(os.Stderr, "  info\tPrint information about a service")
		fmt.Fprintln(os.Stderr, "  help\tPrint interface description or service information")
		fmt.Fprintln(os.Stderr, "  call\tCall a method")
	} else {
		fmt.Fprintln(os.Stderr, "\nOptions:")
		set.PrintDefaults()
	}
	os.Exit(1)
}

func varlink_call(args []string) {
	var err error
	var oneway bool

	callFlags := flag.NewFlagSet("help", flag.ExitOnError)
	callFlags.BoolVar(&oneway, "-oneway", false, "Use bridge for connection")
	var help bool
	callFlags.BoolVar(&help, "help", false, "Prints help information")
	var usage = func() { print_usage(callFlags, "<[ADDRESS/]INTERFACE.METHOD> [ARGUMENTS]") }
	callFlags.Usage = usage

	_ = callFlags.Parse(args)

	if help {
		usage()
	}

	var con *varlink.Connection
	var address string
	var methodName string

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)

		if err != nil {
			ErrPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		address = "bridge:" + bridge
		methodName = callFlags.Arg(0)
	} else {
		uri := callFlags.Arg(0)
		if uri == "" {
			usage()
		}

		li := strings.LastIndex(uri, "/")

		if li == -1 {
			ErrPrintf("Invalid address '%s'\n", uri)
			os.Exit(2)
		}

		address = uri[:li]
		methodName = uri[li+1:]

		con, err = varlink.NewConnection(address)

		if err != nil {
			ErrPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}
	}
	var parameters string
	var params json.RawMessage

	parameters = callFlags.Arg(1)
	if parameters == "" {
		params = nil
	} else {
		json.Unmarshal([]byte(parameters), &params)
	}

	var flags uint64
	flags = 0
	if oneway {
		flags |= varlink.Oneway
	}
	recv, err := con.Send(methodName, params, flags)

	var retval map[string]interface{}

	// FIXME: Use cont
	_, err = recv(&retval)

	f := colorjson.NewFormatter()
	f.Indent = 2
	f.KeyColor = color.New(color.FgCyan)
	f.StringColor = color.New(color.FgMagenta)
	f.NumberColor = color.New(color.FgMagenta)
	f.BoolColor = color.New(color.FgMagenta)
	f.NullColor = color.New(color.FgMagenta)

	if err != nil {
		if e, ok := err.(*varlink.Error); ok {
			ErrPrintf("Call failed with error: %v\n", color.New(color.FgRed).Sprint(e.Name))
			errorRawParameters := e.Parameters.(*json.RawMessage)
			if errorRawParameters != nil {
				var param map[string]interface{}
				_ = json.Unmarshal(*errorRawParameters, &param)
				c, _ := f.Marshal(param)
				fmt.Fprintf(os.Stderr, "%v\n", string(c))
			}
			os.Exit(2)
		}
		ErrPrintf("Error calling '%s': %v\n", methodName, err)
		os.Exit(2)
	}
	c, _ := f.Marshal(retval)
	fmt.Println(string(c))
}

func varlink_help(args []string) {
	var err error

	helpFlags := flag.NewFlagSet("help", flag.ExitOnError)
	var help bool
	helpFlags.BoolVar(&help, "help", false, "Prints help information")
	var usage = func() { print_usage(helpFlags, "<[ADDRESS/]INTERFACE>") }
	helpFlags.Usage = usage

	_ = helpFlags.Parse(args)

	if help {
		usage()
	}

	var con *varlink.Connection
	var address string
	var interfaceName string

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)

		if err != nil {
			ErrPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		address = "bridge:" + bridge
		interfaceName = helpFlags.Arg(0)
	} else {
		uri := helpFlags.Arg(0)
		if uri == "" && bridge == "" {
			ErrPrintf("No ADDRESS or activation or bridge\n\n")
			usage()
		}

		li := strings.LastIndex(uri, "/")

		if li == -1 {
			ErrPrintf("Invalid address '%s'\n", uri)
			os.Exit(2)
		}

		address = uri[:li]

		con, err = varlink.NewConnection(address)

		if err != nil {
			ErrPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}

		interfaceName = uri[li+1:]
	}
	description, err := con.GetInterfaceDescription(interfaceName)

	if err != nil {
		ErrPrintf("Cannot get interface description for '%s': %v\n", interfaceName, err)
		os.Exit(2)
	}

	fmt.Println(description)
}

func varlink_info(args []string) {
	var err error
	infoFlags := flag.NewFlagSet("info", flag.ExitOnError)
	var help bool
	infoFlags.BoolVar(&help, "help", false, "Prints help information")
	var usage = func() { print_usage(infoFlags, "[ADDRESS]") }
	infoFlags.Usage = usage

	_ = infoFlags.Parse(args)

	if help {
		usage()
	}

	var con *varlink.Connection
	var address string

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)

		if err != nil {
			ErrPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		address = "bridge:" + bridge
	} else {
		address = infoFlags.Arg(0)

		if address == "" && bridge == "" {
			ErrPrintf("No ADDRESS or activation or bridge\n\n")
			usage()
		}

		con, err = varlink.NewConnection(address)

		if err != nil {
			ErrPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}
	}

	var vendor, product, version, url string
	var interfaces []string

	err = con.GetInfo(&vendor, &product, &version, &url, &interfaces)

	if err != nil {
		ErrPrintf("Cannot get info for '%s': %v\n", address, err)
		os.Exit(2)
	}

	fmt.Printf("%s %s\n", bold.Sprint("Vendor:"), vendor)
	fmt.Printf("%s %s\n", bold.Sprint("Product:"), product)
	fmt.Printf("%s %s\n", bold.Sprint("Version:"), version)
	fmt.Printf("%s %s\n", bold.Sprint("URL:"), url)
	fmt.Printf("%s\n  %s\n\n", bold.Sprint("Interfaces:"), strings.Join(interfaces[:], "\n  "))
}

func main() {
	var debug bool
	var colorMode string

	flag.CommandLine.Usage = func() { print_usage(nil, "") }
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.StringVar(&bridge, "bridge", "", "Use bridge for connection")
	flag.StringVar(&colorMode, "color", "auto", "colorize output [default: auto]  [possible values: on, off, auto]")

	flag.Parse()

	if colorMode != "on" && (os.Getenv("TERM") == "" || colorMode == "off") {
		color.NoColor = true // disables colorized output
	}

	errorBoldRed = bold.Sprint(color.New(color.FgRed).Sprint("Error:"))

	switch flag.Arg(0) {
	case "info":
		varlink_info(flag.Args()[1:])
	case "help":
		varlink_help(flag.Args()[1:])
	case "call":
		varlink_call(flag.Args()[1:])
	default:
		print_usage(nil, "")
	}
}
