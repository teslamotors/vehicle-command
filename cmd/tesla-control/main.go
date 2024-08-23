package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/shlex"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func writeErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprintf(os.Stderr, "\n")
}

const usage = `
 * Commands sent to a vehicle over the internet require a VIN and a token.
 * Commands sent to a vehicle over BLE require a VIN.
 * Account-management commands require a token.`

func Usage() {
	fmt.Printf("Usage: %s [OPTION...] COMMAND [ARG...]\n", os.Args[0])
	fmt.Printf("\nRun %s help COMMAND for more information. Valid COMMANDs are listed below.", os.Args[0])
	fmt.Println("")
	fmt.Println(usage)
	fmt.Println("")

	fmt.Printf("Available OPTIONs:\n")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Printf("Available COMMANDs:\n")
	maxLength := 0
	var labels []string
	for command := range commands {
		labels = append(labels, command)
		if len(command) > maxLength {
			maxLength = len(command)
		}
	}
	sort.Strings(labels)
	for _, command := range labels {
		info := commands[command]
		fmt.Printf("  %s%s %s\n", command, strings.Repeat(" ", maxLength-len(command)), info.help)
	}
}

func runCommand(acct *account.Account, car *vehicle.Vehicle, args []string, timeout time.Duration) int {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := execute(ctx, acct, car, args); err != nil {
		if protocol.MayHaveSucceeded(err) {
			writeErr("Couldn't verify success: %s", err)
		} else if errors.Is(err, protocol.ErrNoSession) {
			writeErr("You must provide a private key with -key-name or -key-file to execute this command")
		} else {
			writeErr("Failed to execute command: %s", err)
		}
		return 1
	}
	return 0
}

func runInteractiveShell(acct *account.Account, car *vehicle.Vehicle, timeout time.Duration) int {
	scanner := bufio.NewScanner(os.Stdin)
	for fmt.Printf("> "); scanner.Scan(); fmt.Printf("> ") {
		args, err := shlex.Split(scanner.Text())
		if len(args) == 0 {
			continue
		}
		if args[0] == "exit" {
			return 0
		}
		if err != nil {
			writeErr("Invalid command: %s", err)
			continue
		}
		runCommand(acct, car, args, timeout)
	}
	if err := scanner.Err(); err != nil {
		writeErr("Error reading command: %s", err)
		return 1
	}
	return 0
}

func main() {
	status := 1
	defer func() {
		os.Exit(status)
	}()

	var (
		debug          bool
		forceBLE       bool
		commandTimeout time.Duration
		connTimeout    time.Duration
	)
	config, err := cli.NewConfig(cli.FlagAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load credential configuration: %s\n", err)
		os.Exit(1)
	}
	flag.Usage = Usage
	flag.BoolVar(&debug, "debug", false, "Enable verbose debugging messages")
	flag.BoolVar(&forceBLE, "ble", false, "Force BLE connection even if OAuth environment variables are defined")
	flag.DurationVar(&commandTimeout, "command-timeout", 5*time.Second, "Set timeout for commands sent to the vehicle.")
	flag.DurationVar(&connTimeout, "connect-timeout", 20*time.Second, "Set timeout for establishing initial connection.")

	config.RegisterCommandLineFlags()
	flag.Parse()
	if !debug {
		if debugEnv, ok := os.LookupEnv("TESLA_VERBOSE"); ok {
			debug = debugEnv != "false" && debugEnv != "0"
		}
	}
	if debug {
		log.SetLevel(log.LevelDebug)
	}
	config.ReadFromEnvironment()

	args := flag.Args()
	if len(args) > 0 {
		if args[0] == "help" {
			if len(args) == 1 {
				Usage()
				return
			}
			info, ok := commands[args[1]]
			if !ok {
				writeErr("Unrecognized command: %s", args[1])
				return
			}
			info.Usage(args[1])
			status = 0
			return
		} else {
			if err := configureFlags(config, args[0], forceBLE); err != nil {
				writeErr("Missing required flag: %s", err)
				return
			}
		}
	}

	if err := config.LoadCredentials(); err != nil {
		writeErr("Error loading credentials: %s", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
	defer cancel()

	acct, car, err := config.Connect(ctx)
	if err != nil {
		writeErr("Error: %s", err)
		// Error isn't wrapped so we have to check for a substring explicitly.
		if strings.Contains(err.Error(), "operation not permitted") {
			// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
			// heavy-handed way of dealing with devices that are in a bad state.
			writeErr("\nTry again after granting this application CAP_NET_ADMIN:\n\n\tsudo setcap 'cap_net_admin=eip' \"$(which %s)\"\n", os.Args[0])
		}
		return
	}

	if car != nil {
		defer car.Disconnect()
		defer config.UpdateCachedSessions(car)
	}

	if flag.NArg() > 0 {
		status = runCommand(acct, car, flag.Args(), commandTimeout)
	} else {
		status = runInteractiveShell(acct, car, commandTimeout)
	}
}
