# tpsh

A shell for talking ISO 15765-2 (often called ISOTP or CANTP) to a vehicle. Includes decoding of UDS services and Negative Response Codes.

## Dependencies

tpsh uses SocketCAN, and will only work on Linux

tpsh requires the [can-isotp](https://github.com/hartkopp/can-isotp) kernel module

## Usage

```
./tpsh [can device] [transmit ID] [receive ID]
```

Standard addressing example:

```
./tpsh can0 0x7E0 0x7E8
```

Extended addressing ID (ECU 0x10, tester 0xF1)

```
./tpsh can0 0x18DA10F1 0x18DAF110
```

Once started, enter bytes to send at the prompt. These will be framed using ISO 15765-2 and sent using the specified device and IDs.

## Commands

**monitor** or **mon**: enable monitoring, watch for UDS responses from the ECU without sending, use ctrl-c to stop

**tp**: toggle sending of TesterPresent frames

**quit** or **q**: quit