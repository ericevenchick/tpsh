package main

import (
	"fmt"
	"strings"
	"os"
	"encoding/hex"
	"strconv"
	"errors"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/linklayer/go-socketcan/pkg/socketcan"
)

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



var isotp socketcan.Interface
var tp_enabled bool
var tp_stop chan struct{}

func printResponse(resp []byte) {
	for i, b := range resp {
		fmt.Printf("%X ", b)
		if (i+1) % 16 == 0 {
			fmt.Printf("\n")
		}
	}
	fmt.Printf("\n")
	if resp[0] == 0x7F {
		fmt.Printf("NRC: %s (%X)\n", GetNrcName(resp[2]), resp[2])
	}
}

func executor(t string) {
	t = strings.ToLower(t)

	if t == "quit" || t == "q" {
		os.Exit(0)
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
	return []prompt.Suggest{
	}
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
