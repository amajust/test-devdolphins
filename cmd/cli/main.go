package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type OrderRequest struct {
	UserID           string    `json:"user_id"`
	Name             string    `json:"name"`
	Gender           string    `json:"gender"`
	DOB              string    `json:"dob"`
	SelectedServices []Service `json:"selected_services"`
	BasePrice        float64   `json:"base_price"`
	IsR1Eligible     bool      `json:"is_r1_eligible"`
	DiscountPercent  float64   `json:"discount_percent"`
	FinalPrice       float64   `json:"final_price"`
	SimulateFailure  bool      `json:"simulate_failure"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

var medicalServices = map[string][]Service{
	"female": {
		{"Gynecological Checkup", 800},
		{"Mammography", 1500},
		{"General Consultation", 500},
		{"Blood Test - Complete", 600},
		{"Ultrasound", 1200},
		{"Thyroid Function Test", 450},
	},
	"male": {
		{"Prostate Examination", 700},
		{"General Consultation", 500},
		{"Blood Test - Complete", 600},
		{"ECG", 400},
		{"X-Ray Chest", 350},
		{"Lipid Profile", 550},
	},
	"other": {
		{"General Consultation", 500},
		{"Blood Test - Complete", 600},
		{"ECG", 400},
		{"X-Ray Chest", 350},
		{"Ultrasound", 1200},
	},
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║   Medical Clinic Booking System - Event Driven        ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. User Input with Validation

	// Validate Name
	var name string
	for {
		fmt.Print("Enter Name: ")
		name, _ = reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if name != "" {
			break
		}
		fmt.Println("❌ Name cannot be empty. Please try again.")
	}

	// Validate Gender
	var gender string
	for {
		fmt.Print("Enter Gender (Male/Female/Other): ")
		gender, _ = reader.ReadString('\n')
		gender = strings.TrimSpace(gender)
		genderLower := strings.ToLower(gender)
		if genderLower == "male" || genderLower == "female" || genderLower == "other" {
			break
		}
		fmt.Println("❌ Invalid gender. Please enter: Male, Female, or Other")
	}

	// Validate Date of Birth
	var dob string
	var dobDate time.Time
	for {
		fmt.Print("Enter Date of Birth (YYYY-MM-DD): ")
		dob, _ = reader.ReadString('\n')
		dob = strings.TrimSpace(dob)
		var err error
		dobDate, err = time.Parse("2006-01-02", dob)
		if err == nil {
			// Check if date is not in the future
			if dobDate.After(time.Now()) {
				fmt.Println("❌ Date of birth cannot be in the future. Please try again.")
				continue
			}
			break
		}
		fmt.Println("❌ Invalid date format. Please use YYYY-MM-DD (e.g., 1990-05-15)")
	}

	// 2. Display Gender-Specific Medical Services
	fmt.Printf("\n╔════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ Available Medical Services for %s\n", strings.Title(strings.ToLower(gender)))
	fmt.Printf("╚════════════════════════════════════════════════════════╝\n")

	services := medicalServices[strings.ToLower(gender)]
	if services == nil {
		services = medicalServices["other"]
	}

	for i, service := range services {
		fmt.Printf("%d. %-30s ₹%.2f\n", i+1, service.Name, service.Price)
	}

	// 3. User Selects Services
	fmt.Print("\nEnter service numbers separated by commas (e.g., 1,3,4): ")
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	var selectedServices []Service
	var invalidSelections []string

	if selection != "" {
		selectedNums := strings.Split(selection, ",")
		for _, numStr := range selectedNums {
			numStr = strings.TrimSpace(numStr)
			num, err := strconv.Atoi(numStr)
			if err != nil {
				invalidSelections = append(invalidSelections, numStr)
				continue
			}
			if num < 1 || num > len(services) {
				invalidSelections = append(invalidSelections, numStr)
				continue
			}
			selectedServices = append(selectedServices, services[num-1])
		}
	}

	// Show warnings for invalid selections
	if len(invalidSelections) > 0 {
		fmt.Printf("\n⚠️  Skipped invalid selections: %s\n", strings.Join(invalidSelections, ", "))
	}

	if len(selectedServices) == 0 {
		fmt.Println("❌ No valid services selected. Exiting.")
		return
	}

	// Calculate Base Price
	basePrice := 0.0
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║ Selected Services:")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	for _, service := range selectedServices {
		fmt.Printf("  • %-30s ₹%.2f\n", service.Name, service.Price)
		basePrice += service.Price
	}
	fmt.Printf("\n  Base Price (Total): ₹%.2f\n", basePrice)

	// 4. Check R1 Eligibility (Birthday OR Price > ₹1000)
	isR1Eligible, isBirthday := checkR1Eligibility(gender, dob, basePrice)
	discountPercent := 0.0
	finalPrice := basePrice

	if isR1Eligible {
		discountPercent = 12.0
		finalPrice = basePrice * (1 - discountPercent/100)
		fmt.Println("\n✓ Eligible for 12% Discount!")
		if isBirthday {
			fmt.Println("  Reason: Female + Birthday 🎂")
		}
		if basePrice > 1000 {
			fmt.Println("  Reason: High-Value Order (>₹1000)")
		}
		fmt.Printf("  Discount Amount: ₹%.2f\n", basePrice-finalPrice)
		fmt.Printf("  Final Price: ₹%.2f\n", finalPrice)
	} else {
		fmt.Println("\n✗ Not eligible for discount")
		fmt.Println("  (Requires: Female + Birthday OR Total > ₹1000)")
	}

	// 5. Submit Request
	fmt.Print("\n╔════════════════════════════════════════════════════════╗\n")
	fmt.Print("║ Submit Booking Request? (y/n): ")
	confirm, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		fmt.Println("Booking cancelled.")
		return
	}

	// Chaos Testing Option
	fmt.Print("[TEST] Simulate Payment Failure? (y/n): ")
	simFailIn, _ := reader.ReadString('\n')
	simFail := strings.ToLower(strings.TrimSpace(simFailIn)) == "y"

	// 6. Call Order Service
	req := OrderRequest{
		UserID:           strings.ReplaceAll(strings.ToLower(name), " ", "_"),
		Name:             name,
		Gender:           gender,
		DOB:              dob,
		SelectedServices: selectedServices,
		BasePrice:        basePrice,
		IsR1Eligible:     isR1Eligible,
		DiscountPercent:  discountPercent,
		FinalPrice:       finalPrice,
		SimulateFailure:  simFail,
	}
	body, _ := json.Marshal(req)

	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║ Processing Request...")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println("⏳ Sending request to Order Service...")

	resp, err := http.Post("http://localhost:8081/order", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("❌ Error contacting server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 7. Display Result
	var result OrderResponse
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║ BOOKING RESULT")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("Order ID:     %s\n", result.OrderID)
	fmt.Printf("Status:       %s\n", result.Status)
	fmt.Printf("Message:      %s\n", result.Message)

	if result.Status == "CONFIRMED" {
		fmt.Printf("\n✓ Booking Confirmed!\n")
		fmt.Printf("  Reference ID: %s\n", result.OrderID)
		fmt.Printf("  Final Amount: ₹%.2f\n", finalPrice)
	} else {
		fmt.Printf("\n❌ Booking Failed\n")
	}
}

func checkR1Eligibility(gender, dob string, basePrice float64) (bool, bool) {
	isBirthday := false
	isFemale := strings.ToLower(gender) == "female"

	// Check if today is birthday
	dobDate, err := time.Parse("2006-01-02", dob)
	if err == nil {
		today := time.Now()
		if dobDate.Month() == today.Month() && dobDate.Day() == today.Day() {
			isBirthday = true
		}
	}

	// R1: (Female AND Birthday) OR (Price > ₹1000)
	return (isFemale && isBirthday) || (basePrice > 1000), isBirthday
}
