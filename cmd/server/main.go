package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/infrastructure/persistence"
	"salt/hackethon/internal/contexts/registration/transport/api"
	"salt/hackethon/internal/platform/config"
	"salt/hackethon/internal/platform/database"
	"salt/hackethon/internal/platform/mailer"
)

func main() {
	cfg := config.Load()

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	migrations := []string{
		"migrations/0001_create_workshop_registrations.sql",
		"migrations/0002_add_verification_to_workshop_registrations.sql",
	}
	for _, path := range migrations {
		if err := database.Migrate(db, path); err != nil {
			log.Fatalf("migrate %s: %v", path, err)
		}
	}

	repo := persistence.NewRepository(db)
	registerHandler := application.NewRegisterHandler(repo, cfg.QuotaTotal, cfg.VerificationTTL)
	availabilityHandler := application.NewAvailabilityHandler(repo, cfg.QuotaTotal)
	verifyHandler := application.NewVerifyHandler(repo)

	var mail api.Mailer
	if cfg.SMTPUsername != "" && cfg.SMTPPassword != "" {
		mail = mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		})
	} else {
		log.Println("SMTP_USERNAME/SMTP_PASSWORD not set — verification emails will not be sent, only returned as regis_url")
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api.NewHandler(registerHandler, availabilityHandler, verifyHandler, mail).RegisterRoutes(router.Group("/api"))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()
	log.Printf("server listening on :%s", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
	log.Println("server stopped")
}
