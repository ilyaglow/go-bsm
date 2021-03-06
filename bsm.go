// Parse BSM files
package bsm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
)

type empty interface{} // generic type for generator

// ArgToken32bit (or 'arg' token) contains information
// about arguments of the system call.
// These arguments are encoded in 32 bit
type ArgToken32bit struct {
	TokenID       byte   // Token ID (1 byte): 0x2d
	ArgumentID    uint8  // argument ID/number (1 byte)
	ArgumentValue uint32 // argument value (4 bytes)
	Length        uint16 // length of the text (2 bytes)
	Text          string // the string including nul (Length + 1 NUL bytes)
}

// ArgToken64bit (or 'arg' token) contains information
// about arguments of the system call.
// These arguments are encoded in 32 bit
type ArgToken64bit struct {
	TokenID       byte   // Token ID (1 byte): 0x71
	ArgumentID    uint8  // argument ID/number (1 byte)
	ArgumentValue uint64 // argument value (8 bytes)
	Length        uint16 // length of the text (2 bytes)
	Text          string // the string including nul (Length + 1 NUL bytes)
}

// ArbitraryDataToken (or 'arbitrary data' token) contains a byte stream
// of opaque (untyped) data. The size of the data is calculated as the size
// of each unit of data multiplied by the number of units of data.  A
// 'How to print' field is present to specify how to print the data, but
// interpretation of that field is not currently defined.
type ArbitraryDataToken struct {
	TokenID    byte     // token ID (1 byte): 0x21
	HowToPrint byte     // user-defined printing information (1 byte)
	BasicUnit  uint8    // size of a unit in bytes (1 byte)
	UnitCount  uint8    // number if units of data present (1 byte)
	DataItems  [][]byte // user data
}

// AttributeToken32bit (or 'attribute' token) describes the attributes of a file
// associated with the audit event. As files may be identified by 0, 1, or many	path
// names, a path name is not included with the attribute block for a file;  optional
// 'path' tokens may also be present in an audit record indicating which path, if
// any, was used to reach the object. The device number is stored using 32 bit.
// TODO: check if token ID may be 0x31
type AttributeToken32bit struct {
	TokenID          byte   // Token ID (1 byte): 0x3e
	FileAccessMode   uint32 // mode_t associated with file (4 bytes)
	OwnerUserID      uint32 // uid_t associated with file (4 bytes)
	OwnerGroupID     uint32 // gid_t associated with file (4 bytes)
	FileSystemID     uint32 // fsid_t associated with file (4 bytes)
	FileSystemNodeID uint64 // ino_t associated with file (8 bytes)
	Device           uint32 // Device major/minor number (4 bytes)
}

// AttributeToken64bit (or 'attribute' token) describes the attributes of a file
// associated with the audit event. As files may be identified by 0, 1, or many	path
// names, a path name is not included with the attribute block for a file;  optional
// 'path' tokens may also be present in an audit record indicating which path, if
// any, was used to reach the object. The device number is stored using 64 bit.
type AttributeToken64bit struct {
	TokenID          byte   // Token ID (1 byte): 0x73
	FileAccessMode   uint32 // mode_t associated with file (4 bytes)
	OwnerUserID      uint32 // uid_t associated with file (4 bytes)
	OwnerGroupID     uint32 // gid_t associated with file (4 bytes)
	FileSystemID     uint32 // fsid_t associated with file (4 bytes)
	FileSystemNodeID uint64 // ino_t associated with file (8 bytes)
	Device           uint64 // Device major/minor number (8 bytes)
}

// ExecArgsToken (or 'exec_args' token) contains information about
// arguments of the exec() system call.
type ExecArgsToken struct {
	TokenID byte     // Token ID (1 byte): 0x3c
	Count   uint32   // number of arguments (4 bytes)
	Text    []string // Count NUL-terminated strings
}

// ExecEnvToken (or 'exec_env' token) contains current environment
// variables to an exec() system call.
type ExecEnvToken struct {
	TokenID byte     // Token ID (1 byte): 0x3d
	Count   uint32   // number of variables (4 bytes)
	Text    []string // Count NUL-terminated strings
}

// ExitToken (or 'exit' token) contains process
// exit/return code information.
type ExitToken struct {
	TokenID     byte   // Token ID (1 byte): 0x52
	Status      uint32 // Process status on exit (4 bytes)
	ReturnValue int32  // Process return value on exit (4 bytes)
}

// FileToken (or 'file' token) is used at the beginning and end of an audit
// log file to indicate when the audit log begins and ends. It includes a
// pathname so that, if concatenated together, original file boundaries are
// still observable, and gaps in the audit log can be identified.
// BUG: unable to determine token ID (0x11 vs. 0x78 vs . ?)
type FileToken struct {
	TokenID        byte   // Token ID (1 byte):
	Seconds        uint32 // file timestamp (4 bytes)
	Microseconds   uint32 // file timestamp (4 bytes)
	FileNameLength uint16 // file name of audit trail (2 bytes)
	PathName       string // file name of audit trail (FileNameLength + 1 (NULL))
}

// GroupsToken (or 'groups' token) contains a list of group IDs associated
// with the audit event.
type GroupsToken struct {
	TokenID        byte     // Token ID (1 byte): 0x34
	NumberOfGroups uint16   // Number of groups in token (2 bytes)
	GroupList      []uint32 // List of N group IDs (N*4 bytes)
}

// HeaderToken32bit (or 'header' token is used to mark the beginning of a
// complete audit record, and includes the length of the total record in bytes,
// a version number for the record layout, the event type and subtype, and the
// time at which the event occurred. This type uses 32 bits to encode time
// information.
type HeaderToken32bit struct {
	TokenID         byte   // Token ID (1 byte): 0x14
	RecordByteCount uint32 // number of bytes in record (4 bytes)
	VersionNumber   byte   // BSM record version number (1 byte)
	EventType       uint16 // event type (2 bytes)
	EventModifier   uint16 // event sub-type (2 bytes)
	Seconds         uint32 // record time stamp (4 bytes)
	NanoSeconds     uint32 // record time stamp (4 bytes)
}

// HeaderToken64bit (or 'header' token) is used to mark the beginning of a
// complete audit record, and includes the length of the total record in
// bytes, a version number for the record layout, the event type and subtype,
// and the time at which the event occurred. This type uses 64 bits to
// encode time information.
type HeaderToken64bit struct {
	TokenID         byte   // Token ID (1 byte): 0x74
	RecordByteCount uint32 // number of bytes in record (4 bytes)
	VersionNumber   byte   // BSM record version number (1 byte)
	EventType       uint16 // event type (2 bytes)
	EventModifier   uint16 // event sub-type (2 bytes)
	Seconds         uint64 // record time stamp (8 bytes)
	NanoSeconds     uint64 // record time stamp (8 bytes)
}

// ExpandedHeaderToken32bit (or 'expanded header' token) is an expanded
// version of the 'header' token, with the addition of a machine IPv4 or
// IPv6 address. This type uses 32 bits to encode time information.
type ExpandedHeaderToken32bit struct {
	TokenID         byte   // Token ID (1 byte): 0x15
	RecordByteCount uint32 // number of bytes in record (4 bytes)
	VersionNumber   byte   // BSM record version number (2 bytes)
	EventType       uint16 // event type (2 bytes)
	EventModifier   uint16 // event sub-type (2 bytes)
	AddressType     uint32 // host address type and length (1 byte in manpage / 4 bytes in Solaris 10)
	MachineAddress  net.IP // IPv4/6 address (4/16 bytes)
	Seconds         uint32 // record time stamp (4 bytes)
	NanoSeconds     uint32 // record time stamp (4 bytes)
}

// ExpandedHeaderToken64bit (or 'expanded header' token) is an expanded
// version of the 'header' token, with the addition of a machine IPv4 or
// IPv6 address. This type uses 64 bits to encode time information.
type ExpandedHeaderToken64bit struct {
	TokenID         byte   // Token ID (1 byte): 0x79
	RecordByteCount uint32 // number of bytes in record (4 bytes)
	VersionNumber   byte   // BSM record version number (2 bytes)
	EventType       uint16 // event type (2 bytes)
	EventModifier   uint16 // event sub-type (2 bytes)
	AddressType     uint32 // host address type and length (1 byte in manpage / 4 bytes in Solaris 10)
	MachineAddress  net.IP // IPv4/6 address (4/16 bytes)
	Seconds         uint64 // record time stamp (8 bytes)
	NanoSeconds     uint64 // record time stamp (8 bytes)
}

// InAddrToken (or 'in_addr' token) holds a (network byte order) IPv4 address.
// BUGS: token layout documented in audit.log(5) appears to be in conflict with the libbsm(3) implementation of au_to_in_addr_ex(3).
type InAddrToken struct {
	TokenID   byte   // Token ID (1 byte): 0x2a
	IpAddress net.IP // IPv4 address (4 bytes)
}

// ExpandedInAddrToken (or 'expanded in_addr' token) holds a
// (network byte order) IPv4 or IPv6 address.
// TODO: determine value indicating address type
// BUGS: token layout documented in audit.log(5) appears to be in conflict with the libbsm(3) implementation of au_to_in_addr_ex(3).
type ExpandedInAddrToken struct {
	TokenID       byte   // Token ID (1 byte): 0x7e
	IpAddressType byte   // type of IP address (libbsm also calls this 'length')
	IpAddress     net.IP // IP address (4/16 bytes)
}

// IpToken (or 'ip' token) contains an IP(v4) packet header in network
// byte order.
type IpToken struct {
	TokenID            byte   // Token ID (1 byte): 0x2b
	VersionAndIHL      uint8  // Version and IP header length (1 byte)
	TypeOfService      byte   // IP TOS field (1 byte)
	Length             uint16 // IP packet length in network byte order (2 bytes)
	ID                 uint16 // IP header ID for reassembly (2 bytes)
	Offset             uint16 // IP fragment offset and flags, network byte order (2 bytes)
	TTL                uint8  // IP Time-to-Live (1 byte)
	Protocol           uint8  // IP protocol number (1 byte)
	Checksum           uint16 // IP header checksum, network byte order (2 bytes)
	SourceAddress      net.IP // IPv4 source address (4 bytes)
	DestinationAddress net.IP // IPv4 destination addess (4 bytes)
}

// IPortToken (or 'iport' token) stores an IP port number in network byte order.
type IPortToken struct {
	TokenID    byte   // Token ID (1 byte): 0x2c
	PortNumber uint16 // Port number in network byte order (2 bytes)
}

// PathToken (or 'path' token) contains a pathname.
type PathToken struct {
	TokenID    byte   // Token ID (1 byte): 0x23
	PathLength uint16 // Length of path in bytes (2 bytes)
	Path       string // Path name (PathLength bytes + 1 NUL)
}

// PathAttrToken (or 'path_attr' token) contains a set of NUL-terminated path names.
// TODO: verify Token ID
type PathAttrToken struct {
	TokenID byte     // Token ID (1 byte): 0x25 ?
	Count   uint16   // Number of NUL-terminated string(s) in token (2 bytes)
	Path    []string // count NUL-terminated string(s)
}

// ProcessToken32bit (or 'process' token) contains a description of the security
// properties of a process involved as the target of an auditable event, such
// as the destination for signal delivery. It should not be confused with the
// 'subject' token, which describes the subject performing an auditable
// event. This includes both the traditional UNIX security properties, such
// as user IDs and group IDs, but also audit information such as the audit
// user ID and session. The terminal port ID is encoded using 32 bit.
type ProcessToken32bit struct {
	TokenID                byte   // Token ID (1 byte): 0x26
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // session ID (4 bytes)
	TerminalPortID         uint32 // terminal port ID (4 byte)
	TerminalMachineAddress net.IP // IP(v4) address of machine (4 bytes)
}

// ProcessToken64bit (or 'process' token) contains a description of the security
// properties of a process involved as the target of an auditable event, such
// as the destination for signal delivery. It should not be confused with the
// 'subject' token, which describes the subject performing an auditable
// event. This includes both the traditional UNIX security properties, such
// as user IDs and group IDs, but also audit information such as the audit
// user ID and session. The terminal port ID is encoded using 64 bit.
type ProcessToken64bit struct {
	TokenID                byte   // Token ID (1 byte): 0x77
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // session ID (4 bytes)
	TerminalPortID         uint64 // terminal port ID (8 byte)
	TerminalMachineAddress net.IP // IP(v4) address of machine (4 bytes)
}

// ExpandedProcessToken32bit (or 'expanded process' token contains the contents
// of the 'process' token, with the addition of a machine address type and
// variable length address storage capable of containing IPv6 addresses.
// The terminal port ID is encoded using 32 bit.
// TODO: check length of IP records (4 bytes for IPv6?)
type ExpandedProcessToken32bit struct {
	TokenID                byte   // Token ID (1 byte): 0x7b
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // session ID (4 bytes)
	TerminalPortID         uint32 // terminal port ID (4 byte)
	TerminalAddressLength  uint32 // length of machine address (4 bytes)
	TerminalMachineAddress net.IP // IP address of machine (4 or 16 bytes)
}

// ExpandedProcessToken64bit (or 'expanded process' token contains the contents
// of the 'process' token, with the addition of a machine address type and
// variable length address storage capable of containing IPv6 addresses.
// The terminal port ID is encoded using 64 bit.
// TODO: check length of IP records (4 bytes for IPv6?)
type ExpandedProcessToken64bit struct {
	TokenID                byte   // Token ID (1 byte): 0x7d
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // session ID (4 bytes)
	TerminalPortID         uint64 // terminal port ID (8 byte)
	TerminalAddressLength  uint32 // length of machine address (4 bytes)
	TerminalMachineAddress net.IP // IP address of machine (4 or 16 bytes)
}

// ReturnToken32bit (or 'return' token) contains a system call or library
// function return condition, including return value and error number
// associated with the global (C) variable errno. This type uses 32 bit
// to encode the return value.
type ReturnToken32bit struct {
	TokenID     byte   // Token ID (1 byte): 0x27
	ErrorNumber uint8  // errno number, or 0 if undefined (1 byte)
	ReturnValue uint32 // return value (4 bytes)
}

// ReturnToken64bit (or 'return' token) contains a system call or library
// function return condition, including return value and error number
// associated with the global (C) variable errno. This type uses 64 bit
// to encode the return value.
type ReturnToken64bit struct {
	TokenID     byte   // Token ID (1 byte): 0x72
	ErrorNumber uint8  // errno number, or 0 if undefined (1 byte)
	ReturnValue uint64 // return value (8 bytes)
}

// SeqToken ('seq' token) contains a unique and monotonically
// increasing audit event sequence ID. Due to the limited range of 32
// bits, serial number arithmetic and caution should be used when
// comparing sequence numbers.
type SeqToken struct {
	TokenID        byte   // Token ID (1 byte): 0x2f
	SequenceNumber uint32 // audit event sequence number
}

// SocketToken (or 'socket' token) contains information about UNIX
// domain and Internet sockets. Each token has four or eight fields.
// Possible values for token IDs:
// * BSM specification: 0x2e
// * inet32 (IPv4) socket: 0x80
// * inet128 (IPv6) socket: 0x81
// * Unix socket: 0x82
// BUG: last sentence is confusing
type SocketToken struct {
	TokenID       byte   // Token ID (1 byte): 0x2e (BSM spec), 0x80 (inet32 socket), 0x81 (inet128 token), 0x82 (Unix token)
	SocketFamily  uint16 // socket family (2 bytes)
	LocalPort     uint16 // local port (2 bytes)
	SocketAddress net.IP // socket address (4 bytes or 8 bytes for inet128 socket)
}

// ExpandedSocketToken (or 'expanded socket' token) contains
// information about IPv4 and IPv6 sockets.
type ExpandedSocketToken struct {
	TokenID         byte   // Token ID (1 byte): 0x7f
	SocketDomain    uint16 // socket domain (2 bytes)
	SocketType      uint16 // socket type (2 bytes)
	AddressType     uint16 // address type (IPv4/IPv6) (2 bytes)
	LocalPort       uint16 // local port (2 bytes)
	LocalIpAddress  net.IP // local IP address (4/16 bytes)
	RemotePort      uint16 // remote port (2 bytes)
	RemoteIpAddress net.IP // remote IP address (4/16 bytes)
}

// SubjectToken32bit (or 'subject' token) contains information on the
// subject performing the operation described by an audit record, and
// includes similar information to that found in the 'process' and
// 'expanded process' tokens.  However, those tokens are used where
// the process being described is the target of the operation, not the
// authorizing party. This type uses 32 bit to encode the terminal port ID.
type SubjectToken32bit struct {
	TokenID                byte   // Token ID (1 byte): 0x24
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // audit session ID (4 bytes)
	TerminalPortID         uint32 // terminal port ID (4 bytes)
	TerminalMachineAddress net.IP // IP address of machine (4 bytes)
}

// SubjectToken64bit (or 'subject' token) contains information on the
// subject performing the operation described by an audit record, and
// includes similar information to that found in the 'process' and
// 'expanded process' tokens.  However, those tokens are used where the
// process being described is the target of the operation, not the
// authorizing party. This type uses 64 bit to encode the terminal port ID.
type SubjectToken64bit struct {
	TokenID                byte   // Token ID (1 byte): 0x75
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // audit session ID (4 bytes)
	TerminalPortID         uint64 // terminal port ID (8 bytes)
	TerminalMachineAddress net.IP // IP address of machine (4 bytes)
}

// ExpandedSubjectToken32bit (or 'expanded subject' token)
// token consists of the same elements as the 'subject' token,
// with the addition of type/length and variable size machine
// address information in the terminal ID.
// This type uses 32 bit to encode the terminal port ID.
type ExpandedSubjectToken32bit struct {
	TokenID                byte   // Token ID (1 byte): 0x7a
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // audit session ID (4 bytes)
	TerminalPortID         uint32 // terminal port ID (4 bytes)
	TerminalAddressLength  uint32 // length of machine address (4 bytes)
	TerminalMachineAddress net.IP // IP address of machine (4/16 bytes)
}

// ExpandedSubjectToken64bit (or 'expanded subject' token)
// token consists of the same elements as the 'subject' token,
// with the addition of type/length and variable size machine
// address information in the terminal ID.
// This type uses 64 bit to encode the terminal port ID.
// TODO: check length of machine address field (4 bytes for IPv6?)
type ExpandedSubjectToken64bit struct {
	TokenID                byte   // Token ID (1 byte): 0x7c
	AuditID                uint32 // audit user ID (4 bytes)
	EffectiveUserID        uint32 // effective user ID (4 bytes)
	EffectiveGroupID       uint32 // effective group ID (4 bytes)
	RealUserID             uint32 // real user ID (4 bytes)
	RealGroupID            uint32 // real group ID (4 bytes)
	ProcessID              uint32 // process ID (4 bytes)
	SessionID              uint32 // audit session ID (4 bytes)
	TerminalPortID         uint64 // terminal port ID (8 bytes)
	TerminalAddressLength  uint8  // length of machine address (1 byte)
	TerminalMachineAddress net.IP // IP address of machine (4/16 bytes)
}

// SystemVIpcToken (or 'System V IPC' token) contains the System V
// IPC message handle, semaphore handle or shared memory handle.
type SystemVIpcToken struct {
	TokenID      byte   // Token ID (1 byte): 0x22
	ObjectIdType uint8  // Object ID (1 byte)
	ObjectID     uint32 // Object ID (4 bytes)
}

// SystemVIpcPermissionToken (or 'System V IPC permission' token)
// contains a System V IPC access permissions.
type SystemVIpcPermissionToken struct {
	TokenID        byte   // Token ID (1 byte): 0x32
	OwnerUserID    uint32 // User ID of IPC owner (4 bytes)
	OwnerGroupID   uint32 // Group ID of IPC owner (4 bytes)
	CreatorUserID  uint32 // User ID of IPC creator (4 bytes)
	CreatorGroupID uint32 //  Group ID of IPC creator (4 bytes)
	AccessMode     uint32 // Access mode (4 bytes)
	SequenceNumber uint32 // Sequence number (4 bytes)
	Key            uint32 // IPC key (4 bytes)
}

// TextToken (or 'text' token) contains a single NUL-terminated text string.
// TODO: check actual length (documentation looks like off-by-one)
type TextToken struct {
	TokenID    byte   // Token ID (1 byte): 0x28
	TextLength uint16 // length of text string including NUL (2 bytes)
	Text       string // Text string incl. NUL (TextLength bytes)
}

// TrailerToken (or 'trailer' terminates) a BSM audit record. This token
// contains a magic number, and length that can be used to validate that
// the record was read properly.
type TrailerToken struct {
	TokenID         byte   // Token ID (1 byte): 0x13
	TrailerMagic    uint16 // trailer magic number (2 bytes): 0xb105
	RecordByteCount uint32 // number of bytes in record (4 bytes)
}

// ZonenameToken (or 'zonename' token) holds a NUL-terminated string
// with the name of the zone or jail from which the record originated.
type ZonenameToken struct {
	TokenID        byte   // Token ID (1 byte): 0x60
	ZonenameLength uint16 // length of zonename string including NUL (2 bytes)
	Zonename       string // Zonename string including NUL
}

// Go has this unexpected behaviour, where Uvarint() aborts
// after reading the first byte if it is 0x00 (no matter
// what comes later) and can eat max 2 bytes. I expected 8 since
// Uvarint() returns a uint64. Anyhow, I decided to roll my own.

// Convert bytes to uint64 (and abstract away some quirks).
func bytesToUint64(input []byte) (uint64, error) {
	if 8 < len(input) {
		return 0, errors.New("more than four bytes given -> risk of overflow")
	}
	result := uint64(0)
	for i := 0; i < len(input); i++ {
		coeff := uint64(input[i])
		exp := float64(len(input) - i - 1)
		powerOf256 := uint64(math.Pow(float64(256), exp))
		result += coeff * powerOf256
	}
	return result, nil
}

// Convert bytes to uint32 (and abstract away some quirks).
func bytesToUint32(input []byte) (uint32, error) {
	if 4 < len(input) {
		return 0, errors.New("more than four bytes given -> risk of overflow")
	}
	result := uint32(0)
	for i := 0; i < len(input); i++ {
		coeff := uint32(input[i])
		exp := float64(len(input) - i - 1)
		powerOf256 := uint32(math.Pow(float64(256), exp))
		result += coeff * powerOf256
	}
	return result, nil
}

// Convert bytes to uint32 (and abstract away some quirks).
func bytesToUint16(input []byte) (uint16, error) {
	if 2 < len(input) {
		return 0, errors.New("more than two bytes given -> risk of overflow")
	}
	result := uint16(0)
	for i := 0; i < len(input); i++ {
		coeff := uint16(input[i])
		exp := float64(len(input) - i - 1)
		powerOf256 := uint16(math.Pow(float64(256), exp))
		result += coeff * powerOf256
	}
	return result, nil
}

// Determine the size (in bytes) of the current token. This is a
// utility function to determine the number of bytes (yet) to read
// from the input buffer. The return values are:
// * size - size of token in bytes
// * moreBytes - number of more bytes to read to make determination
// * err - any error that ocurred
func determineTokenSize(input []byte) (size, moreBytes int, err error) {
	size = 0
	moreBytes = 0
	err = nil

	// simple case and making sure we get a token ID
	if 0 == len(input) {
		moreBytes = 1
		return
	}

	// do magic based on token ID
	switch input[0] {
	case 0x11: // file token -> variable length
		// make sure we have enough bytes of token to
		// determine its length
		if len(input) < (1 + 4 + 4 + 2) {
			// request bytes up & incl. "File name length" field
			moreBytes = (1 + 4 + 4 + 2) - len(input)
			return
		}
		fileNameLength, local_err := bytesToUint16(input[9:11]) // read 2 bytes indicating file name length
		if local_err != nil {
			err = local_err
			return
		}
		size = 1 + 4 + 4 + 2 + int(fileNameLength) + 1 // don't forget NUL
		return
	case 0x13: // trailer token
		size = 1 + 2 + 4
	case 0x14: // 32 bit Header Token
		size = 1 + 4 + 1 + 2 + 2 + 4 + 4
	case 0x15: // expanded 32 bit header token
		if len(input) < 15 {
			// need more bytes to read AdressType field
			moreBytes = 15 - len(input)
			return
		}
		addrlen, cerr := bytesToUint32(input[10:14])
		if cerr != nil {
			err = cerr
			return
		}
		switch addrlen {
		case 4: // IPv4 -> 4 bytes address
			size = 1 + 4 + 1 + 2 + 2 + 4 + 4 + 4 + 4
		case 16: // IPv6 -> 16 bytes address
			size = 1 + 4 + 1 + 2 + 2 + 4 + 16 + 4 + 4
		default:
			err = fmt.Errorf("invalid value (%d) for 'address type' field in 32bit expanded header token", addrlen)
		}
	case 0x21: // arbitrary data token
		if len(input) < 4 {
			// need more bytes to read BasicUnit and UnitCount fields
			moreBytes = 4 - len(input)
			return
		}
		unitSize := input[2]
		unitCount := input[3]
		size = 1 + 1 + 1 + 1 + int(unitSize)*int(unitCount)
	case 0x22: // System V IPC token
		size = 1 + 1 + 4
	case 0x23: // path token
		if len(input) < 3 {
			// need more bytes to read Count field
			moreBytes = 3 - len(input)
			return
		}
		count, cerr := bytesToUint16(input[1:3])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 2 + int(count)
	case 0x24: // 32 bit Subject Token
		size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4
	case 0x25: // path attr token
		if len(input) < 3 {
			// need more bytes to read Count field
			moreBytes = 3 - len(input)
			return
		}
		strCount, cerr := bytesToUint16(input[1:3])
		if cerr != nil {
			err = cerr
			return
		}
		// make sure we have strCount NUL-terminated strings
		// NOTE: this is very crude and does not do a full validation
		//       since it assumes a benevolent byte stream
		if bytes.Count(input[3:], []byte{0x00}) < int(strCount) {
			moreBytes = 1
			return
		}
		size = len(input)
	case 0x26: // 32bit process token
		size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4
	case 0x27: // 32 bit Return Token
		size = 1 + 1 + 4
	case 0x28: // text token
		if len(input) < 3 {
			// need more bytes to read Count field
			moreBytes = 3 - len(input)
			return
		}
		count, cerr := bytesToUint16(input[1:3])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 2 + int(count)
	case 0x2a: // in_addr token
		size = 1 + 4
	case 0x2b: // ip token
		size = 1 + 1 + 1 + 2 + 2 + 2 + 1 + 1 + 2 + 4 + 4
	case 0x2c: // iport token
		size = 1 + 2
	case 0x2d: // 32bit arg token
		if len(input) < 8 {
			// need more bytes to read Length field
			moreBytes = 8 - len(input)
			return
		}
		strlen, cerr := bytesToUint16(input[6:8])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 1 + 4 + 2 + int(strlen)
	case 0x2e: // socket token
		size = 1 + 2 + 2 + 4
	case 0x2f: // seq token
		size = 1 + 4
	case 0x32: // System V IPC permission token
		size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4
	case 0x34: // groups token
		if len(input) < 3 {
			// need more bytes to read Count field
			moreBytes = 3 - len(input)
			return
		}
		count, cerr := bytesToUint16(input[1:3])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 2 + int(count)*4
	case 0x3c: // exec args token
		if len(input) < 5 {
			// need more bytes to read Count field
			moreBytes = 5 - len(input)
			return
		}
		strCount, cerr := bytesToUint32(input[1:5])
		if cerr != nil {
			err = cerr
			return
		}
		// make sure we have strCount NUL-terminated strings
		// NOTE: this is very crude and does not do a full validation
		//       since it assumes a benevolent byte stream
		if bytes.Count(input[5:], []byte{0x00}) < int(strCount) {
			moreBytes = 1
			return
		}
		size = len(input)
	case 0x3d: // exec env token
		if len(input) < 5 {
			// need more bytes to read Count field
			moreBytes = 5 - len(input)
			return
		}
		strCount, cerr := bytesToUint32(input[1:5])
		if cerr != nil {
			err = cerr
			return
		}
		// make sure we have strCount NUL-terminated strings
		// NOTE: this is very crude and does not do a full validation
		//       since it assumes a benevolent byte stream
		if bytes.Count(input[5:], []byte{0x00}) < int(strCount) {
			moreBytes = 1
			return
		}
		size = len(input)
	case 0x3e: // 32bit attribute token
		size = 1 + 4 + 4 + 4 + 4 + 8 + 4
	case 0x52: // exit token
		size = 1 + 4 + 4
	case 0x60: // zone name token
		if len(input) < 3 {
			// need more bytes to read Length field
			moreBytes = 3 - len(input)
			return
		}
		strlen, cerr := bytesToUint16(input[1:3])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 2 + int(strlen)
	case 0x71: // 64 bit arg token
		if len(input) < 12 {
			// need more bytes to read Length field
			moreBytes = 12 - len(input)
			return
		}
		strlen, cerr := bytesToUint16(input[10:12])
		if cerr != nil {
			err = cerr
			return
		}
		size = 1 + 1 + 8 + 2 + int(strlen) + 1
	case 0x72: // 64 bit Return Token
		size = 1 + 1 + 8
	case 0x73: // 64 bit attribute token
		size = 1 + 4 + 4 + 4 + 4 + 8 + 8
	case 0x74: // 64 bit Header Token
		size = 1 + 4 + 1 + 2 + 2 + 8 + 8
	case 0x75: // 64 bit Subject Token
		size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 4
	case 0x77: // 64 bit process token
		size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 8
	case 0x79: // 64 bit expanded header token
		if len(input) < 15 {
			// need more bytes to read AdressType field
			moreBytes = 15 - len(input)
			return
		}
		addrlen, cerr := bytesToUint32(input[10:14])
		if cerr != nil {
			err = cerr
			return
		}
		switch addrlen {
		case 4: // IPv4 -> 4 bytes address
			size = 1 + 4 + 2 + 2 + 2 + 4 + 4 + 8 + 8
		case 16: // IPv6 -> 16 bytes address
			size = 1 + 4 + 2 + 2 + 2 + 4 + 16 + 8 + 8
		default:
			err = fmt.Errorf("invalid value (%d) for 'address type' field in 64bit expanded header token", addrlen)
		}
	case 0x7a: // expanded 32bit subject token
		if len(input) < 37 {
			// need more bytes to read TerminalAddressLength field
			moreBytes = 37 - len(input)
			return
		}
		addrlen, cerr := bytesToUint32(input[33:37])
		if cerr != nil {
			err = cerr
			return
		}
		switch addrlen {
		case 4: // IPv4 -> 4 bytes address
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4
		case 16: // IPv6 -> 16 bytes address
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 16
		default:
			err = fmt.Errorf("invalid value (%d) for 'terminal address length' field in 32bit expanded subject token", addrlen)
		}
	case 0x7b: // 32bit expanded process token
		if len(input) < 37 {
			moreBytes = 37 - len(input)
			return
		}
		addrlen, cerr := bytesToUint32(input[33:37])
		if cerr != nil {
			err = cerr
			return
		}
		switch addrlen {
		case 4: // IPv4
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4
		case 16: // IPv6
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 16
		default:
			err = fmt.Errorf("invalid value (%d) for 'terminal address length' field in 32bit expanded process token", addrlen)
		}
	case 0x7c: // expanded 64bit subject token
		if len(input) < 38 {
			// need more bytes to read TerminalAddressLength field
			moreBytes = 38 - len(input)
			return
		}
		addrlen := input[37]
		switch addrlen {
		case 4: // IPv4 -> 4 bytes for address
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 1 + 4
		case 16: // IPv6 -> 16 bytes for address
			size = 1 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 1 + 16
		default:
			err = fmt.Errorf("invalid value (%d) for 'terminal address length' field in 64bit expanded subject token", addrlen)
		}
	case 0x7e: // expanded in_addr token
		size = 1 + 1 + 16 // libbsm always allocates 16 bytes
	case 0x7f: // expanded socket token
		if len(input) < 7 {
			// need more bytes to read AddressType field
			moreBytes = 7 - len(input)
			return
		}
		addrlen, cerr := bytesToUint16(input[5:7])
		if cerr != nil {
			err = cerr
			return
		}
		switch addrlen {
		case 4: // IPv4 -> 4 bytes for address
			size = 1 + 2 + 2 + 2 + 2 + 4 + 2 + 4
		case 16: // IPv6 -> 16 bytes for address
			size = 1 + 2 + 2 + 2 + 2 + 16 + 2 + 16
		default:
			err = fmt.Errorf("invalid value (%d) for 'address type' field in expanded socket token", addrlen)
		}
	case 0x80: // socket token (inet32)
		size = 1 + 2 + 2 + 4
	case 0x81: // socket token (inet128)
		size = 1 + 2 + 2 + 16
	case 0x82: // FreeBSD socket token
		size = 1 + 2 + 2 + 4
	default:
		err = fmt.Errorf("can't determine the size of the given token (type): 0x%x", input[0])
	}
	return
}

// ParseHeaderToken32bit parses a HeaderToken32bit out of the given bytes.
func ParseHeaderToken32bit(input []byte) (HeaderToken32bit, error) {
	ptr := 0
	token := HeaderToken32bit{}

	// (static) length check
	if len(input) != 18 {
		return token, errors.New("invalid length of 32bit header token")
	}

	// read token ID
	tokenID := input[ptr]
	if tokenID != 0x14 {
		return token, errors.New("token ID mismatch")
	}
	token.TokenID = tokenID
	ptr += 1

	// read record byte count (4 bytes)
	data32, err := bytesToUint32(input[ptr : ptr+4])
	if err != nil {
		return token, err
	}
	token.RecordByteCount = data32
	ptr += 4

	// read BSM version number (1 byte)
	token.VersionNumber = input[ptr]
	ptr += 1

	// read event type (2 bytes)
	data16, err := bytesToUint16(input[ptr : ptr+2])
	if err != nil {
		return token, err
	}
	token.EventType = data16
	ptr += 2

	// read event sub-type / modifier
	data16, err = bytesToUint16(input[ptr : ptr+2])
	if err != nil {
		return token, err
	}
	token.EventModifier = data16
	ptr += 2

	// read seconds
	data32, err = bytesToUint32(input[ptr : ptr+4])
	if err != nil {
		return token, err
	}
	token.Seconds = data32
	ptr += 4

	// read nanoseconds
	data32, err = bytesToUint32(input[ptr : ptr+4])
	if err != nil {
		return token, err
	}
	token.NanoSeconds = data32

	return token, nil
}

// RecordsFromByteInput yields a generator for all records contained
// in the given byte input. This input has to support the Reader interface
// and may be a file or a device.

// TokenFromByteInput converts bytes read from a given input
// to a BSM token.
func TokenFromByteInput(input io.Reader) (empty, error) {
	tokenBuffer := []byte{0x00}

	// read all the info we need
	n, err := input.Read(tokenBuffer[0:1]) // try to use only token ID
	if nil != err {
		return nil, err
	}
	if n != 1 {
		return nil, errors.New("read " + strconv.Itoa(n) + " bytes, but wanted exactly 1")
	}
	bufidx := 1                                                   // index where to fill the buffer
	buflen, increase, err := determineTokenSize(tokenBuffer[0:1]) // read only token ID
	if nil != err {
		return nil, err
	}

	if increase != 0 { // we need more bytes and test again
		// increase token buffer to hold new bytes
		tmp := make([]byte, 1+increase) // we have read one byte already
		copy(tmp, tokenBuffer)
		tokenBuffer = tmp
		for increase > 0 {
			// try to read all bytes
			n, err := input.Read(tokenBuffer[bufidx : bufidx+increase])
			if nil != err {
				return nil, err
			}
			bufidx += n        // move the index the number of bytes read
			if n != increase { // adjust how many more to read
				increase -= n
			} else {
				increase = 0 // no more bytes need to be read
			}
		}
		buflen, increase, err = determineTokenSize(tokenBuffer)
		if nil != err {
			return nil, err
		}
	}
	// read all the (remaining) bytes we need
	tmp := make([]byte, buflen) // increase token buffer to hold new bytes
	copy(tmp, tokenBuffer)
	tokenBuffer = tmp
	n, err = input.Read(tokenBuffer[bufidx:buflen]) // read remaining bytes
	if nil != err {
		return nil, err
	}
	if n != buflen-bufidx {
		return nil, errors.New("read " + strconv.Itoa(n) + " bytes, but wanted exactly " + strconv.Itoa(buflen-bufidx))
	}

	// process the buffer
	switch tokenBuffer[0] {
	case 0x13: // trailer token
		tmagic, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		bcount, err := bytesToUint32(tokenBuffer[3:6])
		if err != nil {
			return nil, err
		}
		return TrailerToken{
			TokenID:         tokenBuffer[0],
			TrailerMagic:    tmagic,
			RecordByteCount: bcount,
		}, nil

	case 0x14: // 32 bit header token
		token, err := ParseHeaderToken32bit(tokenBuffer)
		if err != nil {
			return nil, err
		}
		return token, nil
	case 0x23: // path token
		token := PathToken{
			TokenID: tokenBuffer[0],
		}
		length, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.PathLength = length
		token.Path = string(tokenBuffer[3 : length+2])
		return token, nil

	case 0x24: // 32 bit subject token
		token := SubjectToken32bit{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint32(tokenBuffer[1:5])
		if err != nil {
			return nil, err
		}
		token.AuditID = val

		val, err = bytesToUint32(tokenBuffer[5:9])
		if err != nil {
			return nil, err
		}
		token.EffectiveUserID = val

		val, err = bytesToUint32(tokenBuffer[9:13])
		if err != nil {
			return nil, err
		}
		token.EffectiveGroupID = val

		val, err = bytesToUint32(tokenBuffer[13:17])
		if err != nil {
			return nil, err
		}
		token.RealUserID = val

		val, err = bytesToUint32(tokenBuffer[17:21])
		if err != nil {
			return nil, err
		}
		token.RealGroupID = val

		val, err = bytesToUint32(tokenBuffer[21:25])
		if err != nil {
			return nil, err
		}
		token.ProcessID = val

		val, err = bytesToUint32(tokenBuffer[25:29])
		if err != nil {
			return nil, err
		}
		token.SessionID = val

		val, err = bytesToUint32(tokenBuffer[29:33])
		if err != nil {
			return nil, err
		}
		token.TerminalPortID = val

		token.TerminalMachineAddress = net.IPv4(
			tokenBuffer[33],
			tokenBuffer[34],
			tokenBuffer[35],
			tokenBuffer[36])
		return token, nil

	case 0x27: // 32 bit return token
		rval, err := bytesToUint32(tokenBuffer[2:6])
		if err != nil {
			return nil, err
		}
		return ReturnToken32bit{
			TokenID:     tokenBuffer[0],
			ErrorNumber: tokenBuffer[1],
			ReturnValue: rval,
		}, nil

	case 0x28: // text token
		length, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		return TextToken{
			TokenID:    tokenBuffer[0],
			TextLength: length,
			Text:       string(tokenBuffer[3 : length+2]), // 3 bytes inital offset - 1 NUL byte = 2 bytes
		}, nil

	case 0x2c: // iport token
		port, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		return IPortToken{TokenID: tokenBuffer[0],
			PortNumber: port,
		}, nil

	case 0x2d: // 32bit arg_token
		token := ArgToken32bit{
			TokenID:    tokenBuffer[0],
			ArgumentID: tokenBuffer[1],
		}
		val, err := bytesToUint32(tokenBuffer[2:6])
		if err != nil {
			return nil, err
		}
		token.ArgumentValue = val
		length, err := bytesToUint16(tokenBuffer[6:8])
		if err != nil {
			return nil, err
		}
		token.Length = length
		token.Text = string(tokenBuffer[8 : length+7])
		return token, nil

	case 0x2e: // socket soken
		token := SocketToken{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.SocketFamily = val
		val, err = bytesToUint16(tokenBuffer[3:5])
		if err != nil {
			return nil, err
		}
		token.LocalPort = val
		token.SocketAddress = net.IPv4(
			tokenBuffer[5],
			tokenBuffer[6],
			tokenBuffer[7],
			tokenBuffer[8])
		return token, nil

	case 0x3e: // 32bit attribute token
		token := AttributeToken32bit{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint32(tokenBuffer[1:5])
		if err != nil {
			return nil, err
		}
		token.FileAccessMode = val
		val, err = bytesToUint32(tokenBuffer[5:9])
		if err != nil {
			return nil, err
		}
		token.OwnerUserID = val
		val, err = bytesToUint32(tokenBuffer[9:13])
		if err != nil {
			return nil, err
		}
		token.OwnerGroupID = val
		val, err = bytesToUint32(tokenBuffer[13:17])
		if err != nil {
			return nil, err
		}
		token.FileSystemID = val
		fsval, err := bytesToUint64(tokenBuffer[17:25])
		if err != nil {
			return nil, err
		}
		token.FileSystemNodeID = fsval
		val, err = bytesToUint32(tokenBuffer[25:29])
		if err != nil {
			return nil, err
		}
		token.Device = val
		return token, nil

	case 0x52: // exit token
		token := ExitToken{
			TokenID: tokenBuffer[0],
		}
		stat, err := bytesToUint32(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.Status = stat
		rval, _ := binary.Varint(tokenBuffer[3:7])
		token.ReturnValue = int32(rval)
		return token, nil

	case 0x60: // zonename token
		token := ZonenameToken{
			TokenID: tokenBuffer[0],
		}
		length, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.ZonenameLength = length
		token.Zonename = string(tokenBuffer[3 : length+2])
		return token, nil

	case 0x73: // 64 bit attribute token
		token := AttributeToken64bit{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint32(tokenBuffer[1:5])
		if err != nil {
			return nil, err
		}
		token.FileAccessMode = val
		val, err = bytesToUint32(tokenBuffer[5:9])
		if err != nil {
			return nil, err
		}
		token.OwnerUserID = val
		val, err = bytesToUint32(tokenBuffer[9:13])
		if err != nil {
			return nil, err
		}
		token.OwnerGroupID = val
		val, err = bytesToUint32(tokenBuffer[13:17])
		if err != nil {
			return nil, err
		}
		token.FileSystemID = val
		bval, err := bytesToUint64(tokenBuffer[17:25])
		if err != nil {
			return nil, err
		}
		token.FileSystemNodeID = bval
		bval, err = bytesToUint64(tokenBuffer[25:33])
		if err != nil {
			return nil, err
		}
		token.Device = bval
		return token, nil

	case 0x7a: // expanded 32bit subject token
		token := ExpandedSubjectToken32bit{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint32(tokenBuffer[1:5])
		if err != nil {
			return nil, err
		}
		token.AuditID = val

		val, err = bytesToUint32(tokenBuffer[5:9])
		if err != nil {
			return nil, err
		}
		token.EffectiveUserID = val

		val, err = bytesToUint32(tokenBuffer[9:13])
		if err != nil {
			return nil, err
		}
		token.EffectiveGroupID = val

		val, err = bytesToUint32(tokenBuffer[13:17])
		if err != nil {
			return nil, err
		}
		token.RealUserID = val

		val, err = bytesToUint32(tokenBuffer[17:21])
		if err != nil {
			return nil, err
		}
		token.RealGroupID = val

		val, err = bytesToUint32(tokenBuffer[21:25])
		if err != nil {
			return nil, err
		}
		token.ProcessID = val

		val, err = bytesToUint32(tokenBuffer[25:29])
		if err != nil {
			return nil, err
		}
		token.SessionID = val

		val, err = bytesToUint32(tokenBuffer[29:33])
		if err != nil {
			return nil, err
		}
		token.TerminalPortID = val

		val, err = bytesToUint32(tokenBuffer[33:37])
		if err != nil {
			return nil, err
		}
		token.TerminalAddressLength = val

		switch val {
		case 4:
			token.TerminalMachineAddress = net.IPv4(
				tokenBuffer[37],
				tokenBuffer[38],
				tokenBuffer[39],
				tokenBuffer[40])
		case 16:
			token.TerminalMachineAddress = tokenBuffer[37:53]
		default:
			return nil, errors.New("can't process length of terminal machine address")
		}
		return token, nil

	case 0x7b: // 32bit expanded process token
		token := ExpandedProcessToken32bit{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint32(tokenBuffer[1:5])
		if err != nil {
			return nil, err
		}
		token.AuditID = val

		val, err = bytesToUint32(tokenBuffer[5:9])
		if err != nil {
			return nil, err
		}
		token.EffectiveUserID = val

		val, err = bytesToUint32(tokenBuffer[9:13])
		if err != nil {
			return nil, err
		}
		token.EffectiveGroupID = val

		val, err = bytesToUint32(tokenBuffer[13:17])
		if err != nil {
			return nil, err
		}
		token.RealUserID = val

		val, err = bytesToUint32(tokenBuffer[17:21])
		if err != nil {
			return nil, err
		}
		token.RealGroupID = val

		val, err = bytesToUint32(tokenBuffer[21:25])
		if err != nil {
			return nil, err
		}
		token.ProcessID = val

		val, err = bytesToUint32(tokenBuffer[25:29])
		if err != nil {
			return nil, err
		}
		token.SessionID = val

		val, err = bytesToUint32(tokenBuffer[29:33])
		if err != nil {
			return nil, err
		}
		token.TerminalPortID = val

		val, err = bytesToUint32(tokenBuffer[33:37])
		if err != nil {
			return nil, err
		}
		token.TerminalAddressLength = val

		switch token.TerminalAddressLength {
		case 4:
			token.TerminalMachineAddress = net.IPv4(
				tokenBuffer[37],
				tokenBuffer[38],
				tokenBuffer[39],
				tokenBuffer[40],
			)
		case 16:
			token.TerminalMachineAddress = tokenBuffer[37:53]
		default:
			return nil, errors.New("invalid value for address length in 32bit expanded process token")
		}
		return token, nil

	case 0x80: // inet32 socket soken
		token := SocketToken{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.SocketFamily = val
		val, err = bytesToUint16(tokenBuffer[3:5])
		if err != nil {
			return nil, err
		}
		token.LocalPort = val
		token.SocketAddress = net.IPv4(
			tokenBuffer[5],
			tokenBuffer[6],
			tokenBuffer[7],
			tokenBuffer[8])
		return token, nil

	case 0x81: // inet128 socket soken
		token := SocketToken{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.SocketFamily = val
		val, err = bytesToUint16(tokenBuffer[3:5])
		if err != nil {
			return nil, err
		}
		token.LocalPort = val
		token.SocketAddress = tokenBuffer[5:21]
		return token, nil

	case 0x82: // FreeBSD socket token
		token := SocketToken{
			TokenID: tokenBuffer[0],
		}
		val, err := bytesToUint16(tokenBuffer[1:3])
		if err != nil {
			return nil, err
		}
		token.SocketFamily = val
		val, err = bytesToUint16(tokenBuffer[3:5])
		if err != nil {
			return nil, err
		}
		token.LocalPort = val

		token.SocketAddress = net.IPv4(
			tokenBuffer[5],
			tokenBuffer[6],
			tokenBuffer[7],
			tokenBuffer[8],
		)
		return token, nil

	default:
		return nil, fmt.Errorf("new token ID found: 0x%x", tokenBuffer[0])
	}
	return nil, nil
}

// BsmRecord represents a BSM record.
type BsmRecord struct {
	Seconds     uint64  // record time stamp (8 bytes)
	NanoSeconds uint64  // record time stamp (8 bytes)
	Tokens      []empty // generic list of all tokens
}

// ParsingResult encapsulates the result of the parsing
// process to be used in conjunction with channels.
type ParsingResult struct {
	Record BsmRecord
	Error  error
}

// ReadBsmRecord read a complete BSM record from the given byte source.
// TODO: support potential file token at the beginning of a stream
// TODO: check record size for consistency
func ReadBsmRecord(input io.Reader) (BsmRecord, error) {
	rec := BsmRecord{}

	// start: header token
	header, err := TokenFromByteInput(input)
	if err != nil {
		return rec, err
	}

	switch v := header.(type) {
	case HeaderToken32bit:
		rec.Seconds = uint64(v.Seconds)
		rec.NanoSeconds = uint64(v.NanoSeconds)
	case HeaderToken64bit:
		rec.Seconds = v.Seconds
		rec.NanoSeconds = v.NanoSeconds
	case ExpandedHeaderToken32bit:
		rec.Seconds = uint64(v.Seconds)
		rec.NanoSeconds = uint64(v.NanoSeconds)
	case ExpandedHeaderToken64bit:
		rec.Seconds = v.Seconds
		rec.NanoSeconds = v.NanoSeconds
	default:
		return rec, errors.New("no header token found")
	}

	nextToken, err := TokenFromByteInput(input)
	if err != nil {
		return rec, err
	}

	_, isEnd := nextToken.(TrailerToken) // assert next token to be trailer and check success
	for !isEnd {
		// append the current token to list (in record)
		rec.Tokens = append(rec.Tokens, nextToken)

		// check if the next (trailer) token indicates the end of record
		nextToken, err = TokenFromByteInput(input)
		if err != nil {
			return rec, err
		}
		_, isEnd = nextToken.(TrailerToken) // assert next token to be trailer and check success
	}

	return rec, nil
}

// RecordGenerator yields a continous stream of BSM records
// until the source is exhausted.
func RecordGenerator(input io.Reader) chan ParsingResult {
	resChan := make(chan ParsingResult)

	// cookie-cutter iterator
	go func() {
		for { // extraction loop
			rec, err := ReadBsmRecord(input)
			res := ParsingResult{
				Record: rec,
				Error:  err,
			}
			resChan <- res
			// leave source is exhausted
			if res.Error == io.EOF {
				break
			}
		}
		close(resChan)
	}()

	return resChan
}
