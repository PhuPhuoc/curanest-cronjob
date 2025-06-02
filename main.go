package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type Appointment struct {
	ID               uuid.UUID  `json:"id"`
	ServiceID        uuid.UUID  `json:"service-id"`
	SvcPackageID     uuid.UUID  `json:"svcpackage-id"`
	CusPackageID     uuid.UUID  `json:"cuspackage-id"`
	NursingID        *uuid.UUID `json:"nursing-id"`
	PatientID        uuid.UUID  `json:"patient-id"`
	PatientAddress   string     `json:"patient-address"`
	PatientLatLng    string     `json:"patient-lat-lng"`
	EstDate          time.Time  `json:"est-date"`
	ActDate          *string    `json:"act-date"`
	Status           string     `json:"status"`
	IsPaid           bool       `json:"is-paid"`
	TotalEstDuration int        `json:"total-est-duration"`
	CreatedAt        time.Time  `json:"created-at"`
}

type AppointmentResponse struct {
	Data []Appointment `json:"data"`
}

type relativesIDResponse struct {
	Data struct {
		RelativesID uuid.UUID `json:"relatives-id"`
	} `json:"data"`
	Success bool `json:"success"`
}

var baseAPIURL string

func fetchAppointments() ([]Appointment, error) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	url := fmt.Sprintf(
		"%s/appointment/api/v1/appointments?est-date-from=%s&est-date-to=%s&apply-paging=false",
		baseAPIURL, today, tomorrow,
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

func sendNotification(accountID, content, subID, route string) error {
	url := fmt.Sprintf("%s/notification/external/rpc/notifications", baseAPIURL)
	body := map[string]string{
		"account-id": accountID,
		"content":    content,
		"sub-id":     subID,
		"route":      route,
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

func remindNurseAttendAppointment() {
	log.Println("⏰ Scheduled jobs:")
	log.Println("- remindStaffAttendAppointment running...")

	appointments, err := fetchAppointments()
	if err != nil {
		log.Printf("❌ Error fetching appointments: %v", err)
		return
	}

	now := time.Now().UTC()
	log.Println("🕒 Current time:", now)
	log.Println("📅 Total appointments fetched:", len(appointments))

	for _, appt := range appointments {

		if appt.NursingID == nil || appt.Status == "upcoming" {
			// log.Printf("🕒 appt-id: %v - appt.NursingID: %v - status: %v \n - date: %v", appt.ID, appt.NursingID, appt.Status, appt.EstDate)
			continue
		}

		diff := appt.EstDate.Sub(now)
		minutesUntil := int(diff.Minutes())

		// log.Printf("🕒 appt-id: %v - est-date: %v \n diff: %v ~ minutesUntil: %v", appt.ID, appt.EstDate, diff, minutesUntil)

		if minutesUntil > 0 && minutesUntil <= 60 {
			log.Printf("🕒 Current in date appt-id: %v - est-date: %v \n", appt.ID, appt.EstDate)
			err := sendNotification(
				appt.NursingID.String(),
				fmt.Sprintf("Bạn có một cuộc hẹn sẽ bắt đầu sau %d phút nữa, hãy lên đường nào!", minutesUntil),
				appt.ID.String(),
				"/(tabs)/home",
			)
			if err != nil {
				log.Printf("❌ Failed to notify for appointment %s: %v", appt.ID, err)
			} else {
				log.Printf("✅ Notification sent for appointment %s (%d phút nữa)", appt.ID, minutesUntil)
			}
		}
	}
	log.Println("===============================================================")
}

func informServicePayment() {
	log.Println("⏰ Scheduled jobs:")
	log.Println("💰 Running payment reminder job...")

	appointments, err := fetchAppointments()
	if err != nil {
		log.Printf("❌ Error fetching appointments: %v", err)
		return
	}

	now := time.Now().UTC()
	log.Println("🕒 Current time:", now)
	log.Println("📅 Total appointments fetched:", len(appointments))

	for _, appt := range appointments {
		if !now.After(appt.EstDate) && !appt.IsPaid {
			reminderMsg := "Nhắc nhở: bạn có một cuộc hẹn đã được lên lịch nhưng chưa thanh toán.\nVui lòng thanh toán để đảm bảo dịch vụ của bạn."

			relativesId, err := getRelativesId(appt.PatientID)
			if err != nil {
				log.Printf("❌ Failed to get relatives-id of patient: %v", err)
				continue
			}

			err = sendNotification(relativesId.String(), reminderMsg, appt.ID.String(), "/detail-payment")
			if err != nil {
				log.Printf("❌ Failed to send payment reminder for appointment %s: %v", appt.ID, err)
			} else {
				log.Printf("✅ Payment reminder sent for appointment %s", appt.ID)
			}
		}
	}
	log.Println("===============================================================")
}

func getRelativesId(patientId uuid.UUID) (*uuid.UUID, error) {
	ctx := context.Background()
	url := fmt.Sprintf("%s/patient/api/v1/patients/%v/relatives-id", baseAPIURL, patientId)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result relativesIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("API call failed or returned unsuccessful response")
	}

	return &result.Data.RelativesID, nil
}

func main() {
	log.Println("🚀 Starting Cronjob Service")
	_ = godotenv.Load()

	baseAPIURL = os.Getenv("BASE_API_URL")
	if baseAPIURL == "" {
		log.Fatal("❌ BASE_API_URL is not set")
	}

	remindInterval, err := strconv.Atoi(os.Getenv("REMIND_INTERVAL_MINUTES"))
	if err != nil {
		remindInterval = 30
	}

	paymentTime1 := os.Getenv("PAYMENT_TIME_1")
	if paymentTime1 == "" {
		paymentTime1 = "00:00"
	}
	paymentTime2 := os.Getenv("PAYMENT_TIME_2")
	if paymentTime2 == "" {
		paymentTime2 = "06:00"
	}

	// Start scheduler
	s := gocron.NewScheduler(time.UTC)
	s.Every(remindInterval).Minutes().Do(remindNurseAttendAppointment)
	s.Every(1).Day().At(paymentTime1).Do(informServicePayment)
	s.Every(1).Day().At(paymentTime2).Do(informServicePayment)

	s.StartBlocking()
}
