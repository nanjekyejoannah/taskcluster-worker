package metaservice

// Message type codes for websocket messages implementing interactive shell.
//
// When the MetaService wants to execute a shell in the virtual machine, it'll
// send an 'exec-shell' action through the /engine/v1/poll end-point. This
// action specifies the command and arguments to execute. The guest-tools will
// start to execute this command and call back to the MetaService, which will
// upgrade the connection to a websocket.
//
// We will send stdin, stdout, stderr, exit and abort messages over this
// websocket. Messages will all have the form: [type] [data]
//
// Where [type] is a single byte with the value of MessageTypeData
// MessageTypeAbort or MessageTypeExit.
// The data property depends on the [type] of the message, as outlined below.
//
// If [type] is MessageTypeData then
//   [data] = [stream] [payload]
// , where  [stream] is a single byte: StreamStdin, StreamStdout, StreamStderr,
// and [payload] is data from this stream. If [payload] is an empty byte
// sequence this signals the end of the stream.
//
// If [type] is MessageTypeAck then
//   [data] = [stream] [N]
// , where [stream] is a single byte: StreamStdin, StreamStdout, StreamStderr,
// and [N] is a big-endian 32 bit unsigned integer acknowleging the remote
// stream to have processed N bytes.
//
// If [type] is MessageTypeAbort then [data] is empty, this message is used
// to request that the executing be aborted.
//
// If [type] is MessageTypeExit then [data] = [exitCode], where exitCode is a
// single byte 0 (success) or 1 (failed) indicating whether the command
// terminated successfully.
const (
	MessageTypeData  = 0
	MessageTypeAck   = 1
	MessageTypeAbort = 2
	MessageTypeExit  = 3
	StreamStdin      = 0
	StreamStdout     = 1
	StreamStderr     = 2
)