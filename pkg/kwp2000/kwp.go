package kwp2000

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"github.com/roffe/gocan/adapter/passthru"
)

type Client struct {
	deviceID, channelID uint32
	pt                  *passthru.PassThru
	outChan             chan *Message
	inChan              chan *Message
	closed              bool
}

type Message struct {
	Data []byte
}

func New() (*Client, error) {
	pt, err := passthru.New(`C:\Program Files (x86)\Drew Technologies, Inc\J2534\MongoosePro GM II\monpa432.dll`)
	if err != nil {
		return nil, err
	}

	c := &Client{
		deviceID:  1,
		channelID: 1,
		pt:        pt,
		outChan:   make(chan *Message, 1),
		inChan:    make(chan *Message, 10),
	}

	if err := c.init(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) init() error {

	if err := c.pt.PassThruOpen("", &c.deviceID); err != nil {
		return fmt.Errorf("PassThruOpen: %w", err)
	}

	if err := c.pt.PassThruConnect(c.deviceID, passthru.ISO14230_PS, 0x00001000, 10400, &c.channelID); err != nil {
		return fmt.Errorf("PassThruConnect: %w", err)
	}

	opts := &passthru.SCONFIG_LIST{
		NumOfParams: 2,
		Params: []passthru.SCONFIG{
			{
				Parameter: passthru.J1962_PINS,
				Value:     0x0700,
			},
			{
				Parameter: passthru.LOOPBACK,
				Value:     0,
			},
		},
	}
	if err := c.pt.PassThruIoctl(c.channelID, passthru.SET_CONFIG, opts); err != nil {
		return fmt.Errorf("PassThruIoctl set options: %w", err)
	}

	filterID := uint32(0)

	mask := &passthru.PassThruMsg{
		ProtocolID:     passthru.ISO14230_PS,
		DataSize:       4,
		ExtraDataIndex: 4,
		Data:           [4128]byte{0x00, 0x00, 0x00, 0x00},
	}

	pattern := &passthru.PassThruMsg{
		ProtocolID:     passthru.ISO14230_PS,
		DataSize:       4,
		ExtraDataIndex: 4,
		Data:           [4128]byte{0x00, 0x00, 0x00, 0x00},
	}

	if err := c.pt.PassThruStartMsgFilter(c.channelID, passthru.PASS_FILTER, mask, pattern, nil, &filterID); err != nil {
		return fmt.Errorf("PassThruStartMsgFilter: %w", err)
	}

	txMSG := &passthru.PassThruMsg{
		ProtocolID:     passthru.ISO14230_PS,
		DataSize:       4,
		ExtraDataIndex: 4,
		Data:           [4128]byte{0x81, 0x41, 0xF1, 0x81},
	}

	rxMSG := &passthru.PassThruMsg{
		ProtocolID: passthru.ISO14230_PS,
	}

	if err := c.pt.PassThruIoctl(c.channelID, passthru.FAST_INIT, txMSG, rxMSG); err != nil {
		return fmt.Errorf("PassThruIoctl fast init: %w", err)
	}

	log.Println(rxMSG.String())
	time.Sleep(1 * time.Second)
	go c.run()
	return nil
}

func (c *Client) run() {
	for !c.closed {
		select {
		case msg := <-c.outChan:
			dataLen := uint32(len(msg.Data))
			txMSG := &passthru.PassThruMsg{
				ProtocolID:     passthru.ISO14230_PS,
				DataSize:       dataLen,
				ExtraDataIndex: dataLen,
				Data:           [4128]byte{0x82, 0x41, 0xF1, 0x1A, 0x90},
			}
			copy(txMSG.Data[:], msg.Data)

			if err := c.pt.PassThruWriteMsgs(c.channelID, uintptr(unsafe.Pointer(txMSG)), 1, 500); err != nil {
				log.Println(fmt.Errorf("PassThruWriteMsgs: %w", err))
			}
		default:
		}
		noMessages := uint32(2)
		rxMSGS := [2]passthru.PassThruMsg{}

		if err := c.pt.PassThruReadMsgs(c.channelID, uintptr(unsafe.Pointer(&rxMSGS)), &noMessages, 900); err != nil {
			if strings.Contains(err.Error(), "Zero messages received") {
				continue
			}
			log.Println(fmt.Errorf("PassThruReadMsgs: %w", err))
			continue
		}

		for i := 0; i < int(noMessages); i++ {
			if rxMSGS[i].DataSize == 0 {
				continue
			}
			//log.Println(string(rxMSGS[i].Data[5:rxMSGS[i].DataSize]))
			select {
			case c.inChan <- &Message{Data: rxMSGS[i].Data[:rxMSGS[i].DataSize]}:
			default:
				log.Println("inChan full, dropping message")
			}

		}

	}
}

func (c *Client) Send(data []byte) {
	c.outChan <- &Message{Data: data}
}

func (c *Client) Read(ctx context.Context) *Message {
	select {
	case msg := <-c.inChan:
		return msg
	case <-ctx.Done():
		return nil
	}
}

func (c *Client) Close() error {
	c.closed = true
	time.Sleep(300 * time.Millisecond)
	txMSG := &passthru.PassThruMsg{
		DataSize:       4,
		ExtraDataIndex: 4,
		Data:           [4128]byte{0x81, 0x41, 0xF1, 0x82},
	}
	txMSG.DataSize = 4
	txMSG.ExtraDataIndex = 4
	if err := c.pt.PassThruWriteMsgs(c.channelID, uintptr(unsafe.Pointer(txMSG)), 1, 500); err != nil {
		return fmt.Errorf("PassThruWriteMsgs: %w", err)
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

/*
func (c *Client) ReadVIN() (string, error) {
	txMSG2 := &passthru.PassThruMsg{
		ProtocolID:     passthru.ISO14230_PS,
		DataSize:       5,
		ExtraDataIndex: 5,
		Data:           [4128]byte{0x82, 0x41, 0xF1, 0x1A, 0x90},
	}

	if err := c.pt.PassThruWriteMsgs(c.channelID, uintptr(unsafe.Pointer(txMSG2)), 1, 500); err != nil {
		return "", fmt.Errorf("PassThruWriteMsgs: %w", err)
	}

	noMessages := uint32(2)
	rxMSGS := [2]passthru.PassThruMsg{}

	if err := c.pt.PassThruReadMsgs(c.channelID, uintptr(unsafe.Pointer(&rxMSGS)), &noMessages, 900); err != nil {
		return "", fmt.Errorf("PassThruReadMsgs: %w", err)
	}

	for i := 0; i < int(noMessages); i++ {
		if rxMSGS[i].DataSize == 0 {
			continue
		}
		log.Println(rxMSGS[i].String())
		return string(rxMSGS[i].Data[5:rxMSGS[i].DataSize]), nil
	}
	return "", errors.New("could not read vin")
}
*/
