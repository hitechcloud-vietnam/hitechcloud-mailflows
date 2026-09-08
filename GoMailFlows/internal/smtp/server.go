package smtp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hitechcloud/mailflows/internal/config"
	"github.com/hitechcloud/mailflows/internal/models"
	"gorm.io/gorm"
)

// Server implements a full SMTP server for receiving and sending email
type Server struct {
	cfg      *config.SMTPConfig
	db       *gorm.DB
	listener net.Listener
}

func NewServer(cfg *config.SMTPConfig, db *gorm.DB) *Server {
	return &Server{
		cfg: cfg,
		db:  db,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	log.Printf("SMTP server starting on %s", addr)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start SMTP server: %w", err)
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("SMTP accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	session := &Session{
		conn: conn,
		db:   s.db,
		cfg:  s.cfg,
	}

	// Send greeting
	session.writeResponse(220, "HiTechCloud MailFlows ESMTP ready")

	reader := newLineReader(conn)
	for {
		line, err := reader.readLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("SMTP read error: %v", err)
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cmd, args := parseCommand(line)
		switch strings.ToUpper(cmd) {
		case "EHLO", "HELO":
			session.handleEHLO(args)
		case "STARTTLS":
			session.handleSTARTTLS()
		case "AUTH":
			session.handleAUTH(args)
		case "MAIL":
			session.handleMAIL(args)
		case "RCPT":
			session.handleRCPT(args)
		case "DATA":
			session.handleDATA(reader)
		case "RSET":
			session.handleRSET()
		case "NOOP":
			session.writeResponse(250, "OK")
		case "QUIT":
			session.writeResponse(221, "Bye")
			return
		default:
			session.writeResponse(500, "Unknown command")
		}
	}
}

// Session represents an SMTP session
type Session struct {
	conn       net.Conn
	db         *gorm.DB
	cfg        *config.SMTPConfig
	authenticated bool
	from       string
	recipients  []string
	messageData []byte
	helloHost   string
}

func (s *Session) writeResponse(code int, message string) {
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		if i < len(lines)-1 {
			fmt.Fprintf(s.conn, "%d-%s\r\n", code, line)
		} else {
			fmt.Fprintf(s.conn, "%d %s\r\n", code, line)
		}
	}
}

func (s *Session) handleEHLO(host string) {
	s.helloHost = host
	s.writeResponse(250, fmt.Sprintf("HiTechCloud MailFlows greets %s\nSIZE %d\n8BITMIME\nSMTPUTF8\nSTARTTLS\nAUTH PLAIN LOGIN", host, s.cfg.MaxMessageSize))
}

func (s *Session) handleSTARTTLS() {
	// TODO: Implement STARTTLS
	s.writeResponse(220, "Ready to start TLS")
}

func (s *Session) handleAUTH(args string) {
	// TODO: Implement authentication
	parts := strings.Fields(args)
	if len(parts) == 0 {
		s.writeResponse(501, "AUTH mechanism required")
		return
	}

	mechanism := strings.ToUpper(parts[0])
	switch mechanism {
	case "PLAIN":
		// TODO: Decode and verify credentials
		s.authenticated = true
		s.writeResponse(235, "Authentication successful")
	case "LOGIN":
		// TODO: Implement LOGIN auth
		s.writeResponse(504, "LOGIN not yet implemented")
	default:
		s.writeResponse(504, "Unsupported authentication mechanism")
	}
}

func (s *Session) handleMAIL(args string) {
	// Parse MAIL FROM:<address>
	if !s.authenticated && s.cfg.AuthRequired {
		s.writeResponse(530, "Authentication required")
		return
	}

	from := extractAddress(args)
	if from == "" {
		s.writeResponse(501, "Invalid MAIL FROM syntax")
		return
	}

	s.from = from
	s.recipients = nil
	s.writeResponse(250, "OK")
}

func (s *Session) handleRCPT(args string) {
	if s.from == "" {
		s.writeResponse(503, "MAIL FROM required first")
		return
	}

	to := extractAddress(args)
	if to == "" {
		s.writeResponse(501, "Invalid RCPT TO syntax")
		return
	}

	// Check if recipient domain is handled by this server
	// For external domains, we relay (if permitted)
	s.recipients = append(s.recipients, to)
	s.writeResponse(250, "OK")
}

func (s *Session) handleDATA(reader *lineReader) {
	if len(s.recipients) == 0 {
		s.writeResponse(503, "RCPT TO required first")
		return
	}

	s.writeResponse(354, "End data with <CR><LF>.<CR><LF>")

	// Read message data
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	data, err := reader.readData(ctx, s.cfg.MaxMessageSize)
	if err != nil {
		s.writeResponse(552, "Message too large or read error")
		return
	}

	s.messageData = data

	// TODO: Process and deliver the message
	// 1. Check spam (optional)
	// 2. Store in recipient mailbox or relay
	// 3. Send to external SMTP if needed

	log.Printf("Received email from %s to %v (%d bytes)", s.from, s.recipients, len(data))

	s.writeResponse(250, "Message accepted")
	s.from = ""
	s.recipients = nil
	s.messageData = nil
}

func (s *Session) handleRSET() {
	s.from = ""
	s.recipients = nil
	s.messageData = nil
	s.writeResponse(250, "OK")
}

// ============================================================
// Line Reader
// ============================================================

type lineReader struct {
	conn   net.Conn
	buffer []byte
}

func newLineReader(conn net.Conn) *lineReader {
	return &lineReader{
		conn:   conn,
		buffer: make([]byte, 0, 4096),
	}
}

func (r *lineReader) readLine() (string, error) {
	for {
		// Check if we have a complete line
		if idx := indexOfCRLF(r.buffer); idx >= 0 {
			line := string(r.buffer[:idx])
			r.buffer = r.buffer[idx+2:]
			return line, nil
		}

		// Read more data
		buf := make([]byte, 4096)
		n, err := r.conn.Read(buf)
		if err != nil {
			return "", err
		}
		r.buffer = append(r.buffer, buf[:n]...)
	}
}

func (r *lineReader) readData(ctx context.Context, maxSize int64) ([]byte, error) {
	var data []byte
	var prev []byte

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		buf := make([]byte, 4096)
		n, err := r.conn.Read(buf)
		if err != nil {
			return nil, err
		}

		chunk := buf[:n]
		data = append(data, chunk...)

		if int64(len(data)) > maxSize {
			return nil, fmt.Errorf("message exceeds maximum size")
		}

		// Check for end of data marker: \r\n.\r\n
		combined := append(prev, chunk...)
		if idx := indexOf(combined, []byte("\r\n.\r\n")); idx >= 0 {
			// Trim to the end marker
			totalLen := len(data) - len(combined) + idx
			return data[:totalLen], nil
		}

		if len(chunk) >= 4 {
			prev = chunk[len(chunk)-4:]
		} else {
			prev = combined
		}
	}
}

func indexOfCRLF(data []byte) int {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func indexOf(data, pattern []byte) int {
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func parseCommand(line string) (string, string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func extractAddress(args string) string {
	start := strings.Index(args, "<")
	end := strings.Index(args, ">")
	if start >= 0 && end > start {
		return args[start+1 : end]
	}
	// Try without angle brackets
	parts := strings.Fields(args)
	for _, p := range parts {
		if strings.Contains(p, "@") {
			return strings.Trim(p, "<>")
		}
	}
	return ""
}
