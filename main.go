package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/linklayer/go-socketcan/pkg/socketcan"
)

var Services = map[string]byte{
	// KWP2000
	"StartDiagnosticSession":                 0x10,
	"ECUReset":                               0x11,
	"ReadFreezeFrameData":                    0x12,
	"ReadDiagnosticTroubleCodes":             0x13,
	"ClearDiagnosticInformation":             0x14,
	"ReadStatusOfDiagnosticTroubleCodes":     0x17,
	"ReadDiagnosticTroubleCodesByStatus":     0x18,
	"ReadECUIdentification":                  0x1a,
	"StopDiagnosticSession":                  0x20,
	"ReadDataByLocalIdentifier":              0x21,
	"ReadDataByCommonIdentifier":             0x22,
	"ReadMemoryByAddress":                    0x23,
	"SetDataRates":                           0x26,
	"SecurityAccess":                         0x27,
	"DynamicallyDefineLocalIdentifier":       0x2c,
	"WriteDataByCommonIdentifier":            0x2e,
	"InputOutputControlByCommonIdentifier":   0x2f,
	"InputOutputControlByLocalIdentifier":    0x30,
	"StartRoutineByLocalIdentifier":          0x31,
	"StopRoutineByLocalIdentifier":           0x32,
	"RequestRoutineResultsByLocalIdentifier": 0x33,
	"RequestDownload":                        0x34,
	"RequestUpload":                          0x35,
	"TransferData":                           0x36,
	"RequestTransferExit":                    0x37,
	"StartRoutineByAddress":                  0x38,
	"StopRoutineByAddress":                   0x39,
	"RequestRoutineResultsByAddress":         0x3a,
	"WriteDataByLocalIdentifier":             0x3b,
	"WriteMemoryByAddress":                   0x3d,
	"TesterPresent":                          0x3e,
	"EscCode":                                0x80,
	// UDS Specific,
	"CommunicationControl": 0x28,
}
var NegativeResponseCodes = map[string]byte{
	"positiveResponse":                       0x00,
	"generalReject":                          0x10,
	"serviceNotSupported":                    0x11,
	"subFunctionNotSupported":                0x12,
	"incorrectMessageLengthOrInvalidFormat":  0x13,
	"responseTooLong":                        0x14,
	"busyRepeatRequest":                      0x21,
	"conditionsNotCorrect":                   0x22,
	"requestSequenceError":                   0x24,
	"requestOutOfRange":                      0x31,
	"securityAccessDenied":                   0x33,
	"invalidKey":                             0x35,
	"exceedNumberOfAttempts":                 0x36,
	"requiredTimeDelayNotExpired":            0x37,
	"uploadDownloadNotAccepted":              0x70,
	"transferDataSuspended":                  0x71,
	"generalProgrammingFailure":              0x72,
	"wrongBlockSequenceCounter":              0x73,
	"responsePending":                        0x78,
	"subFunctionNotSupportedInActiveSession": 0x7E,
	"serviceNotSupportedInActiveSession":     0x7F,
	"rpmTooHigh":                             0x81,
	"rpmTooLow":                              0x82,
	"engineIsRunning":                        0x83,
	"engineIsNotRunning":                     0x84,
	"engineRunTimeTooLow":                    0x85,
	"temperatureTooHigh":                     0x86,
	"temperatureTooLow":                      0x87,
	"vehicleSpeedTooHigh":                    0x88,
	"vehicleSpeedTooLow":                     0x89,
	"throttle/PedalTooHigh":                  0x8A,
	"throttle/PedalTooLow":                   0x8B,
	"transmissionRangeNotInNeutral":          0x8C,
	"transmissionRangeNotInGear":             0x8D,
	"brakeSwitchNotClosed":                   0x8F,
	"shifterLeverNotInPark":                  0x90,
	"torqueConverterClutchLocked":            0x91,
	"voltageTooHigh":                         0x92,
	"voltageTooLow":                          0x93,
}

func GetNrcName(sid byte) string {
	for k, v := range NegativeResponseCodes {
		if sid == v {
			return k
		}
	}
	return "UnknownNRC"
	return fmt.Sprintf("UnknownNRC($%02X)", sid)
}

func GetServiceName(sid byte) string {
	for k, v := range Services {
		if sid == v {
			return k
		}
	}
	return fmt.Sprintf("UnknownService($%02X)", sid)
}

var isotp socketcan.Interface
var tp_enabled bool
var tp_stop chan struct{}

func printResponse(resp []byte) {
	if resp[0] == 0x7F {
		fmt.Printf("NRC: %s (%X)\n", GetNrcName(resp[2]), resp[2])
	} else {
		fmt.Printf("Service: %s (%X)\n", GetServiceName(resp[0]-0x40), resp[0]-0x40)
	}
	fmt.Println(hex.Dump(resp))
}

func executor(t string) {
	t = strings.ToLower(t)
	t = strings.TrimSpace(t)

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
	return []prompt.Suggest{}
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
