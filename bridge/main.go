package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/companyzero/bisonrelay-bot/bot"
	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

const (
	networkBR     uint16 = 1
	networkMatrix uint16 = 2
)

type mtrxMsg struct {
	Network uint16
	Nick    string
	Msg     string
	Room    string
}

type Bridge struct {
	Room string
}

func realMain() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Setup logging
	logDir := filepath.Join(cfg.DataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "bot.log")
	logFd, err := rotator.New(logPath, 32*1024, true, 0)
	if err != nil {
		return err
	}
	defer logFd.Close()

	logBknd := slog.NewBackend(&logWriter{logFd}, slog.WithFlags(slog.LUTC))
	botLog := logBknd.Logger("BOT")
	gcLog := logBknd.Logger("BRLY")
	mtrxLog := logBknd.Logger("MTRX")
	mtrxLog.SetLevel(slog.LevelDebug)

	bknd := slog.NewBackend(os.Stderr)
	log := bknd.Logger("BRLY")
	log.SetLevel(slog.LevelDebug)

	gcChan := make(chan types.GCReceivedMsg)
	mtrxChan := make(chan mtrxMsg)

	botCfg := bot.Config{
		DataDir: cfg.DataDir,
		Log:     botLog,

		URL:            cfg.URL,
		ServerCertPath: cfg.ServerCertPath,
		ClientCertPath: cfg.ClientCertPath,
		ClientKeyPath:  cfg.ClientKeyPath,

		GCChan: gcChan,
		GCLog:  gcLog,
	}

	mCfg := MatrixClientConfig{
		DataDir:  cfg.DataDir,
		User:     cfg.MatrixUser,
		Password: cfg.MatrixPass,
		Token:    cfg.MatrixToken,
		Proxy:    cfg.MatrixProxy,
		Log:      mtrxLog,
	}
	mc := NewMatrixClient(mCfg)

	bot, err := bot.New(botCfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	findRoom := func(fromRoom string) string {
		return cfg.bridges[fromRoom]
	}

	// Launch handler
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case pm := <-gcChan:
				nick := escapeNick(pm.Nick)
				if pm.Msg == nil {
					gcLog.Tracef("empty message from %v", nick)
					continue
				}

				// send to matrix somehow
				room := findRoom(pm.GcAlias)
				if room == "" {
					gcLog.Errorf("room %v is not bridged", pm.GcAlias)
					continue
				}
				// Upload and post embedded images.
				origMsg := pm.Msg.Message
				msg := replaceEmbeds(origMsg, func(embed embeddedArgs) string {
					err := mc.SendEmbed(ctx, room, embed)
					if err != nil {
						mc.cfg.Log.Errorf("sendembed: %v", err)
					}
					return ""
				})
				if msg != "" {
					msg = fmt.Sprintf("[br] <%v> %v", nick, origMsg)
					mc.SendMessage(ctx, room, msg)
				} else {
					msg = fmt.Sprintf("[br] <%v>", nick)
					mc.SendMessage(ctx, room, msg)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-mtrxChan:
				room := findRoom(m.Room)
				if room == "" {
					mtrxLog.Errorf("room %v is not bridged", m.Room)
					continue
				}

				msg := fmt.Sprintf("[m] <%v> %v", m.Nick, m.Msg)
				if err := bot.SendGC(ctx, room, msg); err != nil {
					gcLog.Errorf("failed to send msg to gc %v: %v", room, err)
				}
			}
		}
	}()

	go func() {
		err := mc.Run(ctx, mtrxChan)
		if err != nil {
			cancel()
			mtrxLog.Errorf("failed to run: %v", err)
		}
	}()

	// TODO - get token from Login?
	mc.Login(ctx, mc.cfg.User, mc.cfg.Password)

	for _, bridge := range cfg.Bridges {
		mc.Join(ctx, bridge[1])
	}

	mc.Status(ctx, "online")

	return bot.Run()
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type logWriter struct {
	r *rotator.Rotator
}

func (l *logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return l.r.Write(p)
}
