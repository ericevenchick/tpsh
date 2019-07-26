package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/containous/yaegi/interp"
	"github.com/containous/yaegi/stdlib"
	"github.com/linklayer/go-socketcan/pkg/socketcan"
)

var Services = map[byte]string{
	0x10: "StartDiagnosticSession",
	0x11: "ECUReset",
	0x12: "ReadFreezeFrameData",
	0x13: "ReadDiagnosticTroubleCodes",
	0x14: "ClearDiagnosticInformation",
	0x17: "ReadStatusOfDiagnosticTroubleCodes",
	0x18: "ReadDiagnosticTroubleCodesByStatus",
	0x1a: "ReadECUIdentification",
	0x20: "StopDiagnosticSession",
	0x21: "ReadDataByLocalIdentifier",
	0x22: "ReadDataByCommonIdentifier",
	0x23: "ReadMemoryByAddress",
	0x26: "SetDataRates",
	0x27: "SecurityAccess",
	0x2c: "DynamicallyDefineLocalIdentifier",
	0x2e: "WriteDataByCommonIdentifier",
	0x2f: "InputOutputControlByCommonIdentifier",
	0x30: "InputOutputControlByLocalIdentifier",
	0x31: "StartRoutineByLocalIdentifier",
	0x32: "StopRoutineByLocalIdentifier",
	0x33: "RequestRoutineResultsByLocalIdentifier",
	0x34: "RequestDownload",
	0x35: "RequestUpload",
	0x36: "TransferData",
	0x37: "RequestTransferExit",
	0x38: "StartRoutineByAddress",
	0x39: "StopRoutineByAddress",
	0x3a: "RequestRoutineResultsByAddress",
	0x3b: "WriteDataByLocalIdentifier",
	0x3d: "WriteMemoryByAddress",
	0x3e: "TesterPresent",
	0x80: "EscCode",
	0x28: "CommunicationControl",
}
var NegativeResponseCodes = map[byte]string{
	0x00: "positiveResponse",
	0x10: "generalReject",
	0x11: "serviceNotSupported",
	0x12: "subFunctionNotSupported",
	0x13: "incorrectMessageLengthOrInvalidFormat",
	0x14: "responseTooLong",
	0x21: "busyRepeatRequest",
	0x22: "conditionsNotCorrect",
	0x24: "requestSequenceError",
	0x31: "requestOutOfRange",
	0x33: "securityAccessDenied",
	0x35: "invalidKey",
	0x36: "exceedNumberOfAttempts",
	0x37: "requiredTimeDelayNotExpired",
	0x70: "uploadDownloadNotAccepted",
	0x71: "transferDataSuspended",
	0x72: "generalProgrammingFailure",
	0x73: "wrongBlockSequenceCounter",
	0x78: "responsePending",
	0x7E: "subFunctionNotSupportedInActiveSession",
	0x7F: "serviceNotSupportedInActiveSession",
	0x81: "rpmTooHigh",
	0x82: "rpmTooLow",
	0x83: "engineIsRunning",
	0x84: "engineIsNotRunning",
	0x85: "engineRunTimeTooLow",
	0x86: "temperatureTooHigh",
	0x87: "temperatureTooLow",
	0x88: "vehicleSpeedTooHigh",
	0x89: "vehicleSpeedTooLow",
	0x8A: "throttle/PedalTooHigh",
	0x8B: "throttle/PedalTooLow",
	0x8C: "transmissionRangeNotInNeutral",
	0x8D: "transmissionRangeNotInGear",
	0x8F: "brakeSwitchNotClosed",
	0x90: "shifterLeverNotInPark",
	0x91: "torqueConverterClutchLocked",
	0x92: "voltageTooHigh",
	0x93: "voltageTooLow",
}

var isotp socketcan.Interface
var tp_enabled bool
var tp_stop chan struct{}

func printResponse(resp []byte) {
	if resp[0] == 0x7F {
		fmt.Printf("NRC: %s (%X)\n", NegativeResponseCodes[resp[2]], resp[2])
	} else {
		fmt.Printf("Service: %s (%X)\n", Services[resp[0]-0x40], resp[0]-0x40)
	}
	fmt.Println(hex.Dump(resp))
}

func executor(t string) {
	t = strings.ToLower(t)
	t = strings.TrimSpace(t)
	args := strings.Split(t, " ")

	if t == "quit" || t == "q" {
		os.Exit(0)
	}

	if t == "monitor" || t == "mon" {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		for {
			select {
			case <-c:
				fmt.Printf("\n")
				return
				break
			default:
				resp, err := isotp.RecvBuf()
				if err == nil {
					printResponse(resp)
				}
			}
		}

	}

	if args[0] == "run" {
		fmt.Println(args[1])
		script, err := ioutil.ReadFile(args[1])
		if err != nil {
			fmt.Println(err)
			return
		}

		i := interp.New(interp.Options{})
		i.Use(stdlib.Symbols)
		_, err = i.Eval(string(script))
		if err != nil {
			fmt.Println(err)
			return
		}

		return
	}

	if t == "tp" {
		if tp_enabled {
			close(tp_stop)
			fmt.Println("stopped tester present")
			tp_enabled = false
		} else {
			tp_stop = make(chan struct{})
			go func() {
				for {
					select {
					case <-tp_stop:
						return
					default:
						// tester present, supress positive response
						isotp.SendBuf([]byte{0x3E, 0x80})
						time.Sleep(2 * time.Second)
					}
				}
			}()
			fmt.Println("enabled tester present")
			tp_enabled = true
		}
		return
	}

	bytes := []byte{}
	for _, s := range strings.Split(t, " ") {
		if len(s) == 1 {
			s = "0" + s
		}
		bs, err := hex.DecodeString(s)
		if err != nil {
			fmt.Println("invalid hex string")
			return
		}
		bytes = append(bytes, bs...)
	}

	isotp.SendBuf(bytes)
	for {
		resp, err := isotp.RecvBuf()
		if resp[0] == 0x7E {
			continue
		}

		if err != nil {
			fmt.Println(err)
		} else {
			printResponse(resp)
		}
		break
	}
}

func completer(d prompt.Document) []prompt.Suggest {
	args := strings.Split(d.TextBeforeCursor(), " ")
	s := []prompt.Suggest{}

	if len(args) == 1 {
		// sort service map to ensure order
		var keys []int
		for k := range Services {
			keys = append(keys, int(k))
		}
		sort.Ints(keys)

		for _, k := range keys {
			s = append(s, prompt.Suggest{Text: fmt.Sprintf("%02X", k), Description: Services[byte(k)]})
		}
		s = append(s, prompt.Suggest{Text: "quit", Description: "Quit application"})
		s = append(s, prompt.Suggest{Text: "monitor", Description: "Monitor for traffic"})
		s = append(s, prompt.Suggest{Text: "tp", Description: "Toggle TesterPresent transmission"})
		return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
	}
	return s
}

func parseCanId(s string) (uint32, error) {
	id, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("invalid CAN identifier: %s", os.Args[1]))
	}
	return uint32(id), nil
}

func main() {
	txId, err := parseCanId(os.Args[2])
	if err != nil {
		panic(err)
	}
	rxId, err := parseCanId(os.Args[3])
	if err != nil {
		panic(err)
	}

	fmt.Printf("tx: 0x%X, rx: 0x%X\n", txId, rxId)

	isotp, err = socketcan.NewIsotpInterface(os.Args[1], rxId, txId)
	if err != nil {
		panic(err)
	}
	err = isotp.SetTxPadding(true, 0xAA)
	if err != nil {
		panic(err)
	}

	err = isotp.SetRecvTimeout(250 * time.Millisecond)
	if err != nil {
		panic(err)
	}

	p := prompt.New(executor, completer)
	p.Run()
}
