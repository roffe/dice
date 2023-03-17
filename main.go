package main

import (
	"context"
	"log"
	"time"

	"github.com/roffe/dice/pkg/kwp2000"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func main() {
	if err := asd(); err != nil {
		log.Fatal(err)
	}
}

func asd() error {
	c, err := kwp2000.New()
	if err != nil {
		return err
	}
	defer c.Close()

	c.Send([]byte{0x82, 0x41, 0xF1, 0x1A, 0x90})
	msg := c.Read(context.Background())
	if msg != nil {
		log.Println(string(msg.Data[5:]))
	} else {
		log.Println("no message received")
	}
	//vin, err := c.ReadVIN()
	//if err != nil {
	//	return err
	//}
	//log.Println(vin)
	time.Sleep(1 * time.Second)
	return nil
}
