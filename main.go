package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-co-op/gocron"
)

type Appointment struct {
	ID               string    `json:"id"`
	ServiceID        string    `json:"service-id"`
	SvcPackageID     string    `json:"svcpackage-id"`
	CusPackageID     string    `json:"cuspackage-id"`
	NursingID        string    `json:"nursing-id"`
	PatientID        string    `json:"patient-id"`
	PatientAddress   string    `json:"patient-address"`
	PatientLatLng    string    `json:"patient-lat-lng"`
	EstDate          time.Time `json:"est-date"`
	ActDate          *string   `json:"act-date"`
	Status           string    `json:"status"`
	TotalEstDuration int       `json:"total-est-duration"`
	CreatedAt        time.Time `json:"created-at"`
}

type AppointmentResponse struct {
	Data []Appointment `json:"data"`
}

func fetchAppointments() ([]Appointment, error) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	url := fmt.Sprintf(
		"https://api.curanest.com.vn/appointment/api/v1/appointments?est-date-from=%s&est-date-to=%s&apply-paging=false",
		today, tomorrow,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AppointmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func sendNotification(accountID, minutes string) error {
	url := "https://api.curanest.com.vn/notification/external/rpc/notifications"

	body := map[string]string{
		"account-id": accountID,
		"content":    fmt.Sprintf("Bạn có một cuộc hẹn sẽ bắt đầu sau %s phút nữa, hãy lên đường nào!", minutes),
		"route":      "/(tabs)/schedule",
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send notification, status: %d", resp.StatusCode)
	}
	return nil
}

func checkAndNotify() {
	log.Println("⏳ Running scheduled job...")

	appointments, err := fetchAppointments()
	if err != nil {
		log.Printf("❌ Error fetching appointments: %v", err)
		return
	}

	now := time.Now().UTC()
	log.Println("🕒 Current time:", now)
	log.Println("📅 Total appointments fetched:", len(appointments))

	for _, appt := range appointments {
		if appt.Status != "upcoming" {
			continue
		}

		diff := appt.EstDate.Sub(now)
		minutesUntil := int(diff.Minutes())

		if minutesUntil > 0 && minutesUntil <= 60 {
			err := sendNotification(appt.NursingID, fmt.Sprintf("%d", minutesUntil))
			if err != nil {
				log.Printf("❌ Failed to notify for appointment %s: %v", appt.ID, err)
			} else {
				log.Printf("✅ Notification sent for appointment %s (%d phút nữa)", appt.ID, minutesUntil)
			}
		}
	}
}

func main() {
	log.Println("🚀 Cron service started")

	s := gocron.NewScheduler(time.UTC)

	s.Every(30).Minutes().Do(checkAndNotify)

	s.StartBlocking()
}
