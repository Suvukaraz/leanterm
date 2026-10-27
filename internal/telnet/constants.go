package telnet

// RFC 854 Telnet Commands
const (
	CmdSE   byte = 240 // Subnegotiation End
	CmdNOP  byte = 241 // No Operation
	CmdDM   byte = 242 // Data Mark
	CmdBRK  byte = 243 // Break
	CmdIP   byte = 244 // Interrupt Process
	CmdAO   byte = 245 // Abort Output
	CmdAYT  byte = 246 // Are You There
	CmdEC   byte = 247 // Erase Character
	CmdEL   byte = 248 // Erase Line
	CmdGA   byte = 249 // Go Ahead
	CmdSB   byte = 250 // Subnegotiation Begin
	CmdWILL byte = 251
	CmdWONT byte = 252
	CmdDO   byte = 253
	CmdDONT byte = 254
	CmdIAC  byte = 255 // Interpret As Command
)

// RFC Telnet Options
const (
	OptBinary        byte = 0  // RFC 856: 8-bit Binary Transmission
	OptEcho          byte = 1  // RFC 857: Echo
	OptSGA           byte = 3  // RFC 858: Suppress Go Ahead
	OptTimingMark    byte = 6  // RFC 860: Timing Mark
	OptTTYPE         byte = 24 // RFC 1091: Terminal Type
	OptNAWS          byte = 31 // RFC 1073: Negotiate About Window Size
	OptTSPEED        byte = 32 // RFC 1079: Terminal Speed
	OptLineMode      byte = 34 // RFC 1184: Linemode
	OptNewEnviron    byte = 39 // RFC 1572: New Environment
)

// Subnegotiation Qualifiers
const (
	SubIS    byte = 0
	SubSEND  byte = 1
	SubINFO  byte = 2
	SubVAR   byte = 0
	SubVALUE byte = 1
	SubESC   byte = 2
	SubUSERVAR byte = 3
)
