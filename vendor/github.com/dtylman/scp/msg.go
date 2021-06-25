package scp

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	//CopyMessage Copy Message Opcode
	CopyMessage = 'C'
	//ErrorMessage Error OpCode
	ErrorMessage = 0x1
	//WarnMessage Warning Opcode
	WarnMessage = 0x2
)

//Message is scp control message
type Message struct {
	Type     byte
	Error    error
	Mode     string
	Size     int64
	FileName string
}

func (m *Message) readByte(reader io.Reader) (byte, error) {
	buff := make([]byte, 1)
	_, err := io.ReadFull(reader, buff)
	if err != nil {
		return 0, err
	}
	return buff[0], nil

}

func (m *Message) readOpCode(reader io.Reader) error {
	var err error
	m.Type, err = m.readByte(reader)
	return err
}

//ReadError reads an error message
func (m *Message) ReadError(reader io.Reader) error {
	msg, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	m.Error = errors.New(strings.TrimSpace(string(msg)))
	return nil
}

func (m *Message) readLine(reader io.Reader) (string, error) {
	line := ""
	b, err := m.readByte(reader)
	if err != nil {
		return "", err
	}
	for b != 10 {
		line += string(b)
		b, err = m.readByte(reader)
		if err != nil {
			return "", err
		}
	}
	return line, nil
}

func (m *Message) readCopy(reader io.Reader) error {
	line, err := m.readLine(reader)
	if err != nil {
		return err
	}
	parts := strings.Split(line, " ")
	if len(parts) < 2 {
		return errors.New("Invalid copy line: " + line)
	}
	m.Mode = parts[0]
	m.Size, err = strconv.ParseInt(parts[1], 10, 0)
	if err != nil {
		return err
	}
	m.FileName = parts[2]
	return nil
}

//ReadFrom reads message from reader
func (m *Message) ReadFrom(reader io.Reader) (int64, error) {
	err := m.readOpCode(reader)
	if err != nil {
		return 0, err
	}
	switch m.Type {
	case CopyMessage:
		err = m.readCopy(reader)
		if err != nil {
			return 0, err
		}
	case ErrorMessage, WarnMessage:
		err = m.ReadError(reader)
		if err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("Unsupported opcode: %v", m.Type)
	}
	return m.Size, nil
}

//NewMessageFromReader constructs a new message from a data in reader
func NewMessageFromReader(reader io.Reader) (*Message, error) {
	m := new(Message)
	_, err := m.ReadFrom(reader)
	if err != nil {
		return nil, err
	}
	return m, nil
}
