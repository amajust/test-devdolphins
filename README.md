# Medical Clinic Booking System - Event-Driven Architecture

An event-driven backend system for medical clinic bookings with transactional workflows, discount management, and SAGA choreography pattern for distributed transaction handling.

## 📋 Table of Contents
- [Overview](#overview)
- [Architecture](#architecture)
- [Business Rules](#business-rules)
- [Tech Stack](#tech-stack)
- [Setup & Installation](#setup--installation)
- [Running the System](#running-the-system)
- [Testing](#testing)
- [Project Structure](#project-structure)

---

## 🎯 Overview

This system implements a medical clinic booking platform where:
- Users select gender-specific medical services
- Automatic discount eligibility checking based on business rules
- Daily discount quota management (100 discounts per day)
- Event-driven architecture with proper compensation logic
- SAGA choreography pattern for distributed transactions

---

## 🏗️ Architecture

### Event-Driven Design
```
┌─────────────┐        ┌──────────────────┐        ┌─────────────────┐
│     CLI     │───────▶│  Order Service   │───────▶│ Discount Service│
│  (Client)   │◀───────│   (Port 8081)    │◀───────│                 │
└─────────────┘        └──────────────────┘        └─────────────────┘
                              │ ▲                           │ ▲
                              ▼ │                           ▼ │
                        ┌─────────────────────────────────────┐
                        │   Firestore Event Store             │
                        │  - OrderCreated                     │
                        │  - DiscountReserved/Rejected        │
                        │  - DiscountRelease (Compensation)   │
                        └─────────────────────────────────────┘
```

### SAGA Choreography Pattern

**Normal Flow (Success):**
```
1. CLI → Order Service: Create booking request
2. Order Service → Event Store: Publish OrderCreated
3. Discount Service ← Event Store: Listen OrderCreated
4. Discount Service: Check R1 eligibility + R2 quota
5. Discount Service → Event Store: Publish DiscountReserved
6. Order Service ← Event Store: Receive DiscountReserved
7. Order Service → CLI: Return CONFIRMED
```

**Compensation Flow (Failure):**
```
1-5. Same as normal flow... DiscountReserved
6. Order Service: Payment processing FAILS
7. Order Service → Event Store: Publish DiscountRelease
8. Discount Service ← Event Store: Receive DiscountRelease
9. Discount Service: COMPENSATE - Decrement quota count
10. Order Service → CLI: Return FAILED (with clear message)
```

### Key Architectural Decisions

1. **Event Store**: Firestore as event log and communication channel
2. **Asynchronous Processing**: Services listen to event snapshots
3. **No Central Orchestrator**: Services react to events independently
4. **Idempotency**: Each service checks if it already processed an event
5. **Transactional Integrity**: Firestore transactions for quota management

---

## 📜 Business Rules

### R1: Discount Eligibility (12% Discount)
Apply 12% discount if **ANY** of these conditions are met:
- **(User is Female AND Today is their Birthday)** OR
- **(Base Price Sum > ₹1000)**

### R2: Daily Discount Quota System-Wide Limit
- Maximum **100 R1 discounts** per day across all users
- Counter tracks R1 discounts granted today
- If quota exhausted → Reject with message: *"Daily discount quota reached. Please try again tomorrow."*
- Quota resets at **midnight IST**
- **Important**: Only R1-eligible requests consume quota; non-eligible orders proceed normally

### Service Pricing
**Female Services:**
- Gynecological Checkup: ₹800
- Mammography: ₹1500
- General Consultation: ₹500
- Blood Test - Complete: ₹600
- Ultrasound: ₹1200
- Thyroid Function Test: ₹450

**Male Services:**
- Prostate Examination: ₹700
- General Consultation: ₹500
- Blood Test - Complete: ₹600
- ECG: ₹400
- X-Ray Chest: ₹350
- Lipid Profile: ₹550

**Other Services:**
- General Consultation: ₹500
- Blood Test - Complete: ₹600
- ECG: ₹400
- X-Ray Chest: ₹350
- Ultrasound: ₹1200

---

## 🛠️ Tech Stack

- **Language**: Go 1.25.6
- **Event Store**: Google Cloud Firestore
- **Cloud Provider**: GCP (Firestore)
- **Logging**: Structured JSON logging (slog)
- **Pattern**: SAGA Choreography

---

## 🚀 Setup & Installation

### Prerequisites
- Go 1.25.6 or higher
- GCP Project with Firestore enabled (project: `devdolphins-93118`)
- Service account JSON key file (place as `service-account.json`)
- Firestore indexes configured (see below)
- Git

### Installation Steps

1. **Clone the repository**
```bash
git clone <repository-url>
cd devdolphintest
```

2. **Install Go dependencies**
```bash
go mod download
```

3. **Set up GCP credentials**
```bash
export GOOGLE_APPLICATION_CREDENTIALS=./service-account.json
```

4. **Configure Firestore Indexes**

The system requires a composite index for querying events:

**Automatic Setup** (Recommended):
- Run the discount service once, it will provide a URL to auto-create the index
- Click the URL in the error message and approve the index creation

**Manual Setup**:
1. Go to Firebase Console: https://console.firebase.google.com/project/devdolphins-93118/firestore/indexes
2. Create Composite Index:
   - Collection: `events`
   - Fields:
     - `type` - Ascending
     - `timestamp` - Ascending
3. Wait 2-5 minutes for index to build

5. **Build the binaries**
```bash
# Create bin directory
mkdir -p bin

# Build all services
go build -o bin/discount-service services/discount/main.go
go build -o bin/order-service services/order/main.go
go build -o bin/cli cmd/cli/main.go
```

---

## 🏃 Running the System

### Start Services

**Terminal 1: Discount Service**
```bash
export GOOGLE_APPLICATION_CREDENTIALS=./service-account.json
./bin/discount-service
```

**Terminal 2: Order Service**
```bash
export GOOGLE_APPLICATION_CREDENTIALS=./service-account.json
./bin/order-service
```

**Terminal 3: CLI Client**
```bash
./bin/cli
```

### Example Usage

```
╔════════════════════════════════════════════════════════╗
║   Medical Clinic Booking System - Event Driven        ║
╚════════════════════════════════════════════════════════╝

Enter Name: Priya Sharma
Enter Gender (Male/Female/Other): Female
Enter Date of Birth (YYYY-MM-DD): 1995-02-01

╔════════════════════════════════════════════════════════╗
║ Available Medical Services for Female
╚════════════════════════════════════════════════════════╝
1. Gynecological Checkup            ₹800.00
2. Mammography                      ₹1500.00
3. General Consultation             ₹500.00
4. Blood Test - Complete            ₹600.00
5. Ultrasound                       ₹1200.00
6. Thyroid Function Test            ₹450.00

Enter service numbers separated by commas: 2,5

╔════════════════════════════════════════════════════════╗
║ Selected Services:
╚════════════════════════════════════════════════════════╝
  • Mammography                      ₹1500.00
  • Ultrasound                       ₹1200.00

  Base Price (Total): ₹2700.00

✓ Eligible for 12% Discount!
  Reason: High-Value Order (>₹1000)
  Discount Amount: ₹324.00
  Final Price: ₹2376.00

╔════════════════════════════════════════════════════════╗
║ Submit Booking Request? (y/n): y
[TEST] Simulate Payment Failure? (y/n): n

╔════════════════════════════════════════════════════════╗
║ Processing Request...
╚════════════════════════════════════════════════════════╝
⏳ Sending request to Order Service...

╔════════════════════════════════════════════════════════╗
║ BOOKING RESULT
╚════════════════════════════════════════════════════════╝
Order ID:     a1b2c3d4-e5f6-7890-abcd-ef1234567890
Status:       CONFIRMED
Message:      Booking confirmed! Final price: ₹2376.00 (12% discount applied)

✓ Booking Confirmed!
  Reference ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
  Final Amount: ₹2376.00
```

---

## 🧪 Testing

### Test Scenarios Overview

This system includes 3 comprehensive end-to-end test scenarios:

1. **Positive Case**: High-value order with discount and quota available
2. **Negative Case 1**: Quota exhausted - rejection without compensation
3. **Negative Case 2**: Payment failure - compensation (SAGA pattern)

---

## Test Scenario 1: POSITIVE - Successful Booking with R1 Discount (High-Value Order)

### Objective
Demonstrate successful booking when user qualifies for R1 discount due to high-value order (>₹1000) and R2 quota is available.

### Test Data
- **Name**: Raj Kumar
- **Gender**: Male
- **Date of Birth**: 1990-05-15
- **Selected Services**:
  1. Prostate Examination (₹700)
  2. Blood Test - Complete (₹600)
- **Base Price**: ₹1300
- **Expected Discount**: 12% (₹156)
- **Final Price**: ₹1144

### Prerequisites
- Daily quota count < 100
- Both services should be running
- GCP Firestore configured

### Execution Steps
```bash
# Start services
./bin/discount-service &
./bin/order-service &

# Run CLI
./bin/cli
```

### Input Sequence
```
Enter Name: Raj Kumar
Enter Gender (Male/Female/Other): Male
Enter Date of Birth (YYYY-MM-DD): 1990-05-15
Enter service numbers separated by commas: 1,2
Submit Booking Request? (y/n): y
[TEST] Simulate Payment Failure? (y/n): n
```

### Expected Outcome
- ✓ Services displayed correctly for Male gender
- ✓ Base price calculated: ₹1300
- ✓ R1 eligibility detected (price > ₹1000)
- ✓ 12% discount applied
- ✓ Order event published
- ✓ Discount service reserves quota (count incremented)
- ✓ Order confirmed with final price ₹1144
- **Status**: CONFIRMED
- **Message**: "Booking confirmed! Final price: ₹1144.00 (12% discount applied)"

### Observable Logs
```json
// Order Service
{"level":"INFO","msg":"Order Received","order_id":"xxx","trace_id":"yyy","user":"Raj Kumar","base_price":1300,"r1_eligible":true,"final_price":1144}
{"level":"INFO","msg":"Order Event Published - Checking R2 Quota","order_id":"xxx","trace_id":"yyy"}

// Discount Service
{"level":"INFO","msg":"Processing R1-Eligible Order","order_id":"xxx","trace_id":"yyy","base_price":1300,"discount":12}
{"level":"INFO","msg":"R2 Quota Reserved","order_id":"xxx","quota_used":1,"quota_remaining":99}

// Order Service
{"level":"INFO","msg":"Discount Reserved","order_id":"xxx","trace_id":"yyy"}
```

---

## Test Scenario 2: NEGATIVE - Daily Quota Exhausted (Triggers Rejection)

### Objective
Demonstrate system behavior when R1 discount is eligible but R2 daily quota (100) is exhausted. This tests the rejection path without needing compensation.

### Test Data
- **Name**: Priya Sharma
- **Gender**: Female
- **Date of Birth**: 1995-02-01 (Birthday TODAY - Feb 1, 2026)
- **Selected Services**:
  1. General Consultation (₹500)
- **Base Price**: ₹500
- **R1 Eligible**: YES (Female + Birthday)
- **Expected Result**: REJECTED (quota exhausted)

### Prerequisites
- Set daily quota count to 100 (run 100 successful bookings first OR manually set in Firestore)
- **To simulate**: Temporarily change QuotaLimit to 1 in discount/main.go, then run this as 2nd request

### Input Sequence
```
Enter Name: Priya Sharma
Enter Gender (Male/Female/Other): Female
Enter Date of Birth (YYYY-MM-DD): 1995-02-01
Enter service numbers separated by commas: 3
Submit Booking Request? (y/n): y
[TEST] Simulate Payment Failure? (y/n): n
```

### Expected Outcome
- ✓ Birthday detected (Feb 1 = today)
- ✓ R1 eligibility confirmed (Female + Birthday)
- ✓ 12% discount calculation shown
- ✓ Order event published
- ✓ Discount service checks quota
- ✓ Quota limit reached (100/100)
- ✗ Discount rejected
- **Status**: REJECTED
- **Message**: "Daily discount quota reached. Please try again tomorrow."
- ✓ NO quota consumed (rejection doesn't increment)
- ✓ NO compensation needed (quota wasn't reserved)

### Observable Logs
```json
// Order Service
{"level":"INFO","msg":"Order Received","order_id":"xxx","trace_id":"yyy","user":"Priya Sharma","r1_eligible":true}
{"level":"INFO","msg":"Order Event Published - Checking R2 Quota"}

// Discount Service
{"level":"INFO","msg":"Processing R1-Eligible Order","order_id":"xxx","base_price":500,"discount":12}
{"level":"INFO","msg":"R2 Quota Exhausted","order_id":"xxx","quota_limit":100,"current_count":100}

// Order Service
{"level":"INFO","msg":"Discount Rejected","order_id":"xxx","reason":"Daily discount quota reached. Please try again tomorrow."}
```

### Business Rule Validation
- R1 discount eligibility correctly identified
- R2 quota enforcement working
- User receives clear rejection message
- System handles quota exhaustion gracefully

---

## Test Scenario 3: NEGATIVE - Payment Failure with Compensation (SAGA Pattern)

### Objective
Demonstrate SAGA choreography pattern with compensation logic when payment fails after discount quota is reserved.

### Test Data
- **Name**: Amit Verma
- **Gender**: Male
- **Date of Birth**: 1988-07-20
- **Selected Services**:
  1. Prostate Examination (₹700)
  2. ECG (₹400)
  3. Lipid Profile (₹550)
- **Base Price**: ₹1650
- **R1 Eligible**: YES (price > ₹1000)
- **Discount**: 12% (₹198)
- **Final Price**: ₹1452
- **Simulate Failure**: YES

### Prerequisites
- Daily quota count < 100
- Services running

### Input Sequence
```
Enter Name: Amit Verma
Enter Gender (Male/Female/Other): Male
Enter Date of Birth (YYYY-MM-DD): 1988-07-20
Enter service numbers separated by commas: 1,4,6
Submit Booking Request? (y/n): y
[TEST] Simulate Payment Failure? (y/n): y  ← IMPORTANT: Select YES
```

### Expected Outcome - Step by Step

#### Phase 1: Initial Processing
- ✓ R1 eligibility confirmed (₹1650 > ₹1000)
- ✓ 12% discount calculated
- ✓ Order event published
- ✓ Discount service reserves quota
- ✓ Quota count incremented (e.g., 5 → 6)
- ✓ DiscountReserved event published

#### Phase 2: Simulated Failure
- ✓ Order service receives DiscountReserved
- ✓ Payment simulation fails (SimulateFailure=true)
- ✗ Payment processing fails

#### Phase 3: COMPENSATION (SAGA)
- ✓ Order service publishes DiscountRelease event
- ✓ Discount service receives DiscountRelease
- ✓ Quota transactionally decremented (6 → 5)
- ✓ System state restored

#### Final Result
- **Status**: FAILED
- **Message**: "Payment processing failed. Discount quota has been released."
- ✓ Quota returned to pool
- ✓ Other users can now use the released quota

### Observable Logs - SAGA Flow
```json
// Order Service - Initial
{"level":"INFO","msg":"Order Received","order_id":"xxx","r1_eligible":true,"base_price":1650}
{"level":"INFO","msg":"Order Event Published - Checking R2 Quota"}

// Discount Service - Reserve
{"level":"INFO","msg":"Processing R1-Eligible Order","order_id":"xxx"}
{"level":"INFO","msg":"R2 Quota Reserved","quota_used":6,"quota_remaining":94}

// Order Service - Failure
{"level":"INFO","msg":"Discount Reserved","order_id":"xxx"}
{"level":"WARN","msg":"Simulating Failure after Reservation","order_id":"xxx"}

// Discount Service - COMPENSATION
{"level":"INFO","msg":"Quota Compensation Executed","order_id":"xxx","new_count":5}
```

### SAGA Pattern Verification
1. **Forward Transaction**: Quota reserved successfully
2. **Failure Detection**: Payment simulation failed
3. **Compensation Action**: DiscountRelease event published
4. **State Rollback**: Quota decremented back
5. **Eventual Consistency**: System state consistent after compensation

### Why This Demonstrates SAGA Choreography
- ✓ **No Central Orchestrator**: Services react to events independently
- ✓ **Event-Driven**: Communication via event store (Firestore)
- ✓ **Compensation Logic**: Automatic quota release on failure
- ✓ **Distributed Transaction**: Spans multiple services
- ✓ **Eventual Consistency**: System eventually reaches consistent state

---

## Quick Test Commands

```bash
# Test 1: Successful booking with discount
echo -e "Raj Kumar\nMale\n1990-05-15\n1,2\ny\nn" | ./bin/cli

# Test 2: Payment failure with compensation (SAGA)
echo -e "Amit Verma\nMale\n1988-07-20\n1,4,6\ny\ny" | ./bin/cli
```

### Additional Test Cases

**Test 4: Non-R1-Eligible Order (No Discount Path)**
- **Scenario**: User selects services totaling ₹600 (not birthday, not female)
- **Expected**: Order completes immediately without quota check
- **Validation**: Discount service logs "Skipping Non-R1-Eligible Order"

**Test 5: Female Birthday with Low-Value Order**
- **Scenario**: Female user, birthday, ₹500 order
- **Expected**: R1 eligible due to birthday condition, quota checked
- **Validation**: Both R1 conditions work independently

---

## Test Results Summary

| Scenario | R1 Eligible | R2 Quota Available | Payment | Result | Compensation |
|----------|-------------|-------------------|---------|--------|--------------|
| Test 1   | ✓ (>₹1000)  | ✓ (< 100)         | ✓       | CONFIRMED | N/A |
| Test 2   | ✓ (Birthday)| ✗ (= 100)         | N/A     | REJECTED | Not Needed |
| Test 3   | ✓ (>₹1000)  | ✓ (< 100)         | ✗       | FAILED | ✓ Quota Released |
| Non-R1   | ✗           | N/A               | ✓       | CONFIRMED | N/A |

---

## 📁 Project Structure

```
devdolphintest/
├── cmd/
│   └── cli/
│       └── main.go                 # Terminal client with service selection
├── services/
│   ├── order/
│   │   └── main.go                 # Order service (port 8081)
│   └── discount/
│       └── main.go                 # Discount service (quota management)
├── pkg/
│   ├── events/
│   │   └── events.go               # Event definitions
│   └── common/
│       └── client.go               # Firestore client factory
├── bin/                            # Compiled binaries
├── service-account.json            # GCP service account credentials
├── go.mod                          # Go dependencies
├── README.md                       # This file
└── TEST_SCENARIOS.md               # Detailed test documentation
```

---

## 🔍 Observability Features

### Structured Logging
All services emit JSON structured logs with:
- **trace_id**: Connects all events for a single request
- **order_id**: Identifies the booking
- **timestamp**: ISO 8601 format
- **level**: INFO, WARN, ERROR

### Distributed Tracing
Follow a request across services:
```bash
# Filter logs by trace_id
cat discount.log order.log | jq 'select(.trace_id == "abc123")'
```

### Event Tracking
All events stored in Firestore with:
- Event type
- Timestamp
- Complete payload
- Trace ID for correlation

---

## 🎓 Key Learning Outcomes

This implementation demonstrates:

1. **Event-Driven Architecture**: Loosely coupled services communicating via events
2. **SAGA Choreography Pattern**: Distributed transactions with compensation logic (DiscountRelease)
3. **Business Rule Implementation**: Complex discount eligibility (R1) and quota management (R2)
4. **Smart Event Publishing**: Only R1-eligible orders use event-driven flow for efficiency
5. **Quota Management**: Date-based automatic reset with IST timezone handling
6. **Observability**: Structured JSON logging with trace IDs for request correlation
7. **Error Handling**: Input validation, graceful failures with clear user messages
8. **Idempotency**: Preventing duplicate event processing with existence checks
9. **Chaos Engineering**: Built-in failure simulation for testing resilience

---

## 🔧 Configuration

### Quota Limit
Edit `services/discount/main.go`:
```go
const QuotaLimit = 100 // Change this value
```

### Timezone
IST (Indian Standard Time) is hardcoded:
```go
const ISTOffset = 5*time.Hour + 30*time.Minute
```

### Ports
- **Order Service**: 8081
- **Discount Service**: (no HTTP endpoint, event-driven only)
- **Firestore Emulator**: 8080

---

## 🐛 Troubleshooting

### Issue: Services can't connect to Firestore
**Solution**: Ensure GCP credentials are set correctly
```bash
export GOOGLE_APPLICATION_CREDENTIALS=./service-account.json
# Verify the file exists
ls -la service-account.json
```

### Issue: "Timeout waiting for discount service"
**Solution**: Check if discount service is running and processing events
```bash
ps aux | grep discount-service
tail -f logs/discount.log
```

### Issue: Quota not resetting at midnight
**Solution**: Quota is date-based (IST). Check date format in Firestore:
```bash
# Document ID format: 2026-02-01
```

---

## 📝 Assumptions & Design Decisions

1. **Service Discovery**: Hardcoded localhost URLs (production would use service mesh/DNS)
2. **Authentication**: Not implemented (would use OAuth/JWT in production)
3. **Rate Limiting**: No client-side rate limits (only quota enforcement)
4. **Database**: Firestore for both events and quota state (could separate in production)
5. **Retry Logic**: No automatic retries (could add exponential backoff)
6. **Event Ordering**: Firestore snapshots maintain order via timestamp
7. **Concurrency**: Firestore transactions handle concurrent quota updates

---

## 📚 References

- [SAGA Pattern](https://microservices.io/patterns/data/saga.html)
- [Event-Driven Architecture](https://martinfowler.com/articles/201701-event-driven.html)
- [Google Firestore Transactions](https://cloud.google.com/firestore/docs/manage-data/transactions)

---

## 👥 Contributing

For improvements or bug fixes:
1. Create feature branch
2. Make changes with tests
3. Submit pull request

---

## 📄 License

[Your License Here]

---

**Built with ❤️ demonstrating modern distributed systems patterns**
