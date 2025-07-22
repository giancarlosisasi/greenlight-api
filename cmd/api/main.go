package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/giancarlosisasi/greenlight-api/internal/data"
	"github.com/giancarlosisasi/greenlight-api/internal/mailer"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn string
	}

	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	mailer *mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// load env vars
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading the .env file")
	}

	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		logger.Error(fmt.Sprintf("error to parse the smtp port, current value is: %s", smtpPortStr))
		panic(fmt.Sprintf("SMTP Port is an invalid int value: %s", smtpPortStr))
	}
	// create the mailer
	mailer := mailer.NewDialer(
		os.Getenv("SMTP_HOST"),
		smtpPort,
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
		os.Getenv("SMTP_SENDER"),
	)

	// rate limit default values
	cfg.limiter.enabled = os.Getenv("LIMITER_ENABLED") == "true"
	cfg.limiter.rps = 2
	cfg.limiter.burst = 4
	rps := os.Getenv("LIMITER_RPS")
	rpsI, err := strconv.Atoi(rps)
	if err == nil {
		cfg.limiter.rps = float64(rpsI)
	} else {
		logger.Warn(fmt.Sprintf("> invalid integer value for env var %s", "LIMITER_RPS"))
	}
	burst := os.Getenv("LIMITER_BURST")
	burstI, err := strconv.Atoi(burst)
	if err == nil {
		cfg.limiter.burst = burstI
	} else {
		logger.Warn(fmt.Sprintf("> invalid integer value for env var %s", "LIMITER_BURST"))
	}

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	// make sure to put the defer close in the root of the application
	// so the db conn is only closed when the app closes
	defer db.Close()
	logger.Info("database connection pool established!")

	app := application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer,
	}

	err = app.serve()

	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg config) (*pgxpool.Pool, error) {
	databaseUrl := os.Getenv("DATABASE_URL")
	pgxConfig, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to parse database url configuration: %v\n", err)
		return nil, err
	}

	pgxConfig.MaxConns = 1
	pgxConfig.MaxConnIdleTime = time.Minute * 15

	dbpool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create connection pool: %v\n", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err = dbpool.Ping(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to the database: %v\n", err)
		return nil, err
	}

	return dbpool, nil
}
