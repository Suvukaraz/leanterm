package telnet

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

type parserState int

const (
	stateNormal parserState = iota
	stateIAC
	stateWill
	stateWont
	stateDo
	stateDont
	stateSB
	stateSubData
	stateSubIAC
	stateCR
)

type Options struct {
	TerminalType string
	NAWS         bool
	LineEnding   string // "crlf", "crnul", "lf"
	EscapeChar   byte   // default 0x1D (^])
	Username     string
	Cols         int
	Rows         int
}

type Client struct {
	conn       net.Conn
	opts       Options
	mu         sync.Mutex
	closed     bool
	ttypeCycle int

	// Option states: 0 = NO, 1 = YES
	usOpts  map[byte]bool // what we are doing (WILL)
	himOpts map[byte]bool // what server is doing (DO)

	// Stream parser state
	state      parserState
	subOpt     byte
	subBuf     []byte
	crPending  bool

	// Escape mode prompt line
	inEscapeMode bool
	escapeBuf    []byte

	// Output buffer for parsed terminal bytes
	outBuf bytes.Buffer
	readCh chan []byte
	errCh  chan error

	// Status message hook for local echo / prompt
	StatusMsg func(msg string)
}

func NewClient(conn net.Conn, opts Options) *Client {
	if opts.TerminalType == "" {
		opts.TerminalType = "XTERM-256COLOR"
	}
	if opts.LineEnding == "" {
		opts.LineEnding = "crlf"
	}
	if opts.EscapeChar == 0 {
		opts.EscapeChar = 0x1D // Ctrl+]
	}
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}

	c := &Client{
		conn:    conn,
		opts:    opts,
		usOpts:  make(map[byte]bool),
		himOpts: make(map[byte]bool),
		readCh:  make(chan []byte, 128),
		errCh:   make(chan error, 1),
	}

	// Start reading from network socket
	go c.readLoop()

	return c
}

func (c *Client) readLoop() {
	buf := make([]byte, 8192)
	for {
		n, err := c.conn.Read(buf)
		if n > 0 {
			c.mu.Lock()
			c.parseIncoming(buf[:n])
			if c.outBuf.Len() > 0 {
				data := make([]byte, c.outBuf.Len())
				copy(data, c.outBuf.Bytes())
				c.outBuf.Reset()
				c.mu.Unlock()
				c.readCh <- data
			} else {
				c.mu.Unlock()
			}
		}
		if err != nil {
			c.errCh <- err
			close(c.readCh)
			break
		}
	}
}

func (c *Client) Read(p []byte) (int, error) {
	data, ok := <-c.readCh
	if !ok {
		select {
		case err := <-c.errCh:
			if err != nil {
				return 0, err
			}
		default:
		}
		return 0, io.EOF
	}
	n := copy(p, data)
	return n, nil
}

func (c *Client) parseIncoming(data []byte) {
	for i := 0; i < len(data); i++ {
		b := data[i]

		switch c.state {
		case stateNormal:
			if b == CmdIAC {
				c.state = stateIAC
			} else if b == '\r' {
				c.state = stateCR
			} else {
				c.outBuf.WriteByte(b)
			}

		case stateCR:
			c.state = stateNormal
			if b == 0 {
				// CR NUL -> CR
				c.outBuf.WriteByte('\r')
			} else if b == '\n' {
				// CR LF -> CR LF
				c.outBuf.WriteString("\r\n")
			} else if b == CmdIAC {
				c.outBuf.WriteByte('\r')
				c.state = stateIAC
			} else {
				c.outBuf.WriteByte('\r')
				c.outBuf.WriteByte(b)
			}

		case stateIAC:
			switch b {
			case CmdIAC:
				// Escaped 255 -> literal 255
				c.outBuf.WriteByte(CmdIAC)
				c.state = stateNormal
			case CmdWILL:
				c.state = stateWill
			case CmdWONT:
				c.state = stateWont
			case CmdDO:
				c.state = stateDo
			case CmdDONT:
				c.state = stateDont
			case CmdSB:
				c.subBuf = c.subBuf[:0]
				c.state = stateSB
			case CmdSE:
				c.state = stateNormal
			default:
				// Consumed commands: NOP, DM, BRK, IP, AO, AYT, EC, EL, GA
				c.state = stateNormal
			}

		case stateWill:
			c.handleWill(b)
			c.state = stateNormal

		case stateWont:
			c.handleWont(b)
			c.state = stateNormal

		case stateDo:
			c.handleDo(b)
			c.state = stateNormal

		case stateDont:
			c.handleDont(b)
			c.state = stateNormal

		case stateSB:
			c.subOpt = b
			c.subBuf = c.subBuf[:0]
			c.state = stateSubData

		case stateSubData:
			if b == CmdIAC {
				c.state = stateSubIAC
			} else {
				c.subBuf = append(c.subBuf, b)
			}

		case stateSubIAC:
			if b == CmdSE {
				c.handleSubnegotiation(c.subOpt, c.subBuf)
				c.state = stateNormal
			} else if b == CmdIAC {
				c.subBuf = append(c.subBuf, CmdIAC)
				c.state = stateSubData
			} else {
				c.state = stateSubData
			}
		}
	}
}

func (c *Client) handleDo(opt byte) {
	switch opt {
	case OptBinary:
		if !c.usOpts[OptBinary] {
			c.usOpts[OptBinary] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptBinary})
		}
	case OptSGA:
		if !c.usOpts[OptSGA] {
			c.usOpts[OptSGA] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptSGA})
		}
	case OptTTYPE:
		if !c.usOpts[OptTTYPE] {
			c.usOpts[OptTTYPE] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptTTYPE})
		}
	case OptNAWS:
		if c.opts.NAWS {
			if !c.usOpts[OptNAWS] {
				c.usOpts[OptNAWS] = true
				_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptNAWS})
				_ = c.sendNAWSLocked(c.opts.Cols, c.opts.Rows)
			}
		} else {
			_ = c.sendRaw([]byte{CmdIAC, CmdWONT, OptNAWS})
		}
	case OptTSPEED:
		if !c.usOpts[OptTSPEED] {
			c.usOpts[OptTSPEED] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptTSPEED})
		}
	case OptNewEnviron:
		if !c.usOpts[OptNewEnviron] {
			c.usOpts[OptNewEnviron] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptNewEnviron})
		}
	case OptTimingMark:
		_ = c.sendRaw([]byte{CmdIAC, CmdWILL, OptTimingMark})
	case OptEcho:
		// Client never offers local echo
		_ = c.sendRaw([]byte{CmdIAC, CmdWONT, OptEcho})
	default:
		// Refuse unknown options
		_ = c.sendRaw([]byte{CmdIAC, CmdWONT, opt})
	}
}

func (c *Client) handleDont(opt byte) {
	if c.usOpts[opt] {
		c.usOpts[opt] = false
		_ = c.sendRaw([]byte{CmdIAC, CmdWONT, opt})
	}
}

func (c *Client) handleWill(opt byte) {
	switch opt {
	case OptBinary:
		if !c.himOpts[OptBinary] {
			c.himOpts[OptBinary] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdDO, OptBinary})
		}
	case OptEcho:
		if !c.himOpts[OptEcho] {
			c.himOpts[OptEcho] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdDO, OptEcho})
		}
	case OptSGA:
		if !c.himOpts[OptSGA] {
			c.himOpts[OptSGA] = true
			_ = c.sendRaw([]byte{CmdIAC, CmdDO, OptSGA})
		}
	default:
		// Refuse options we don't want the server to do
		_ = c.sendRaw([]byte{CmdIAC, CmdDONT, opt})
	}
}

func (c *Client) handleWont(opt byte) {
	if c.himOpts[opt] {
		c.himOpts[opt] = false
		_ = c.sendRaw([]byte{CmdIAC, CmdDONT, opt})
	}
}

func (c *Client) handleSubnegotiation(opt byte, payload []byte) {
	if len(payload) == 0 {
		return
	}

	switch opt {
	case OptTTYPE:
		if payload[0] == SubSEND {
			// RFC 1091 cycling
			var ttype string
			switch c.ttypeCycle {
			case 0:
				ttype = c.opts.TerminalType
			case 1:
				ttype = "XTERM"
			default:
				ttype = "VT100"
			}
			c.ttypeCycle++

			var resp []byte
			resp = append(resp, CmdIAC, CmdSB, OptTTYPE, SubIS)
			resp = append(resp, []byte(ttype)...)
			resp = append(resp, CmdIAC, CmdSE)
			_ = c.sendRaw(resp)
		}

	case OptTSPEED:
		if payload[0] == SubSEND {
			var resp []byte
			resp = append(resp, CmdIAC, CmdSB, OptTSPEED, SubIS)
			resp = append(resp, []byte("38400,38400")...)
			resp = append(resp, CmdIAC, CmdSE)
			_ = c.sendRaw(resp)
		}

	case OptNewEnviron:
		if payload[0] == SubSEND && c.opts.Username != "" {
			var resp []byte
			resp = append(resp, CmdIAC, CmdSB, OptNewEnviron, SubIS)
			resp = append(resp, SubVAR)
			resp = append(resp, []byte("USER")...)
			resp = append(resp, SubVALUE)
			resp = append(resp, []byte(c.opts.Username)...)
			resp = append(resp, CmdIAC, CmdSE)
			_ = c.sendRaw(resp)
		}
	}
}

func (c *Client) sendNAWSLocked(cols, rows int) error {
	var payload []byte
	wHi := byte((cols >> 8) & 0xFF)
	wLo := byte(cols & 0xFF)
	hHi := byte((rows >> 8) & 0xFF)
	hLo := byte(rows & 0xFF)

	payload = append(payload, CmdIAC, CmdSB, OptNAWS)
	for _, b := range []byte{wHi, wLo, hHi, hLo} {
		if b == CmdIAC {
			payload = append(payload, CmdIAC, CmdIAC)
		} else {
			payload = append(payload, b)
		}
	}
	payload = append(payload, CmdIAC, CmdSE)
	return c.sendRaw(payload)
}

func (c *Client) SendNAWS(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opts.Cols = cols
	c.opts.Rows = rows
	if c.usOpts[OptNAWS] {
		return c.sendNAWSLocked(cols, rows)
	}
	return nil
}

func (c *Client) SendCommand(cmd byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendRaw([]byte{CmdIAC, cmd})
}

func (c *Client) sendRaw(data []byte) error {
	if c.closed || c.conn == nil {
		return io.ErrClosedPipe
	}
	_, err := c.conn.Write(data)
	return err
}

// Write processes user keystrokes from terminal view
func (c *Client) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.ErrClosedPipe
	}

	// Check if escape character triggered local prompt
	if c.opts.EscapeChar != 0 && len(data) == 1 && data[0] == c.opts.EscapeChar && !c.inEscapeMode {
		c.inEscapeMode = true
		c.escapeBuf = c.escapeBuf[:0]
		c.outBuf.WriteString("\r\n\x1b[1;36mtelnet>\x1b[0m ")
		// Flush to readCh
		flush := make([]byte, c.outBuf.Len())
		copy(flush, c.outBuf.Bytes())
		c.outBuf.Reset()
		go func() { c.readCh <- flush }()
		return 1, nil
	}

	// While in escape mode, handle local commands
	if c.inEscapeMode {
		return c.handleEscapeInputLocked(data)
	}

	// Regular terminal mode: format output to server
	var payload []byte
	isBinary := c.usOpts[OptBinary]

	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == CmdIAC {
			// Double IAC
			payload = append(payload, CmdIAC, CmdIAC)
		} else if b == '\r' {
			if isBinary {
				payload = append(payload, '\r')
			} else {
				switch c.opts.LineEnding {
				case "crlf":
					payload = append(payload, '\r', '\n')
				case "crnul":
					payload = append(payload, '\r', 0)
				case "lf":
					payload = append(payload, '\n')
				default:
					payload = append(payload, '\r', '\n')
				}
			}
		} else {
			payload = append(payload, b)
		}
	}

	err := c.sendRaw(payload)
	return len(data), err
}

func (c *Client) handleEscapeInputLocked(data []byte) (int, error) {
	for _, b := range data {
		if b == '\r' || b == '\n' {
			cmdStr := strings.TrimSpace(string(c.escapeBuf))
			c.escapeBuf = c.escapeBuf[:0]
			c.outBuf.WriteString("\r\n")

			if cmdStr == "" {
				// Empty line exits escape mode
				c.inEscapeMode = false
			} else {
				c.executeEscapeCommand(cmdStr)
			}

			if c.inEscapeMode {
				c.outBuf.WriteString("\x1b[1;36mtelnet>\x1b[0m ")
			}
		} else if b == 0x1B { // Esc key exits escape mode
			c.inEscapeMode = false
			c.escapeBuf = c.escapeBuf[:0]
			c.outBuf.WriteString("\r\n")
		} else if b == 0x7F || b == 0x08 { // Backspace
			if len(c.escapeBuf) > 0 {
				c.escapeBuf = c.escapeBuf[:len(c.escapeBuf)-1]
				c.outBuf.WriteString("\b \b")
			}
		} else if b >= 32 && b < 127 {
			c.escapeBuf = append(c.escapeBuf, b)
			c.outBuf.WriteByte(b)
		}
	}

	if c.outBuf.Len() > 0 {
		flush := make([]byte, c.outBuf.Len())
		copy(flush, c.outBuf.Bytes())
		c.outBuf.Reset()
		go func() { c.readCh <- flush }()
	}

	return len(data), nil
}

func (c *Client) executeEscapeCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case "close", "quit":
		c.inEscapeMode = false
		_ = c.Close()
	case "status":
		c.outBuf.WriteString(fmt.Sprintf("Connected to %s\r\n", c.conn.RemoteAddr()))
		c.outBuf.WriteString(fmt.Sprintf("Escape character is '%c'\r\n", c.opts.EscapeChar))
	case "send":
		if len(parts) > 1 {
			switch strings.ToLower(parts[1]) {
			case "brk":
				_ = c.sendRaw([]byte{CmdIAC, CmdBRK})
				c.outBuf.WriteString("Sent BREAK\r\n")
			case "ayt":
				_ = c.sendRaw([]byte{CmdIAC, CmdAYT})
				c.outBuf.WriteString("Sent AYT\r\n")
			case "ip":
				_ = c.sendRaw([]byte{CmdIAC, CmdIP})
				c.outBuf.WriteString("Sent Interrupt Process\r\n")
			case "ao":
				_ = c.sendRaw([]byte{CmdIAC, CmdAO})
				c.outBuf.WriteString("Sent Abort Output\r\n")
			case "ec":
				_ = c.sendRaw([]byte{CmdIAC, CmdEC})
				c.outBuf.WriteString("Sent Erase Character\r\n")
			case "el":
				_ = c.sendRaw([]byte{CmdIAC, CmdEL})
				c.outBuf.WriteString("Sent Erase Line\r\n")
			default:
				c.outBuf.WriteString(fmt.Sprintf("Unknown send argument: %s\r\n", parts[1]))
			}
		}
	default:
		c.outBuf.WriteString(fmt.Sprintf("?Invalid command: %s (commands: close, quit, status, send [brk|ayt|ip|ao|ec|el])\r\n", cmd))
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
