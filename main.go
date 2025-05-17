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

func sendNotification(accountID, contentTime string) error {
	url := "https://api.curanest.com.vn/notification/external/rpc/notifications"

	body := map[string]string{
		"account-id": accountID,
		"content":    fmt.Sprintf("Bạn có một cuộc hẹn vào lúc %s, hãy lên đường nào!", contentTime),
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
	vnLoc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	log.Println("Time now - time in current machine: ", now)
	log.Println("Time now - location Ho_Chi_Minh: ", now.In(vnLoc))
	log.Println("Number of appointments at this time: ", len(appointments))

	for _, appt := range appointments {
		if appt.Status == "upcoming" {
			continue
		}
		diff := appt.EstDate.Sub(now)
		if diff > 0 && diff <= time.Hour {
			vnTime := appt.EstDate.In(vnLoc)
			timeStr := vnTime.Format("15:04 02-01-2006")
			err := sendNotification(appt.NursingID, timeStr)
			if err != nil {
				log.Printf("❌ Failed to notify for appointment %s: %v", appt.ID, err)
			} else {
				log.Printf("✅ Notification sent for appointment %s", appt.ID)
			}
		}
	}
}

func main() {
	log.Println("🚀 Cron service started")

	s := gocron.NewScheduler(time.UTC)

	// Job chạy mỗi 30 phút
	s.Every(30).Minutes().Do(checkAndNotify)

	// 👉 Dùng cho testing: mỗi 30 giây (comment dòng trên lại nếu cần test)
	// s.Every(30).Seconds().Do(checkAndNotify)

	s.StartBlocking()
}
