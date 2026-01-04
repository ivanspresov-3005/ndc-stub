package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

/*
ITERATION 1:
- Add Version attr
- Add Document/Name
- Add Party/Sender/TravelAgencySender/Name

ITERATION 2:
- Add Travelers with PTC
- Respond with DataLists/PassengerList and IDs

ITERATION 3:
- Respond with DataLists/FlightSegmentList + PaxJourneyList
- OfferItem references Journey + Passenger(s)

ITERATION 4:
- Replace OfferItem/TotalAmount with OfferItem/PriceDetail
- PriceDetail contains BaseAmount, Taxes/TotalTaxAmount, TotalAmount

ITERATION 5:
- Add DataLists/ServiceDefinitionList (catalog of ancillaries)
- Add OfferItem/Services referencing ServiceDefinitionRef
- Add a priced BAG service

ITERATION 6:
- Add OfferPriceRQ/OfferPriceRS endpoint
- Store last AirShoppingRS offer in memory (very simple stub state)
- Validate OfferRef + OfferItemRef
- Return OfferPriceRS with same Offer, marked priced/confirmed

ITERATION 7:
- Add OrderCreateRQ endpoint
- Validate OfferRef + OfferItemRef against stored offer
- Return OrderViewRS containing an Order created from the selected OfferItem
- Keep the same "in-memory state" logic as Iteration 6

ITERATION 8:
- Add OrderRetrieveRQ endpoint: POST /ndc/orderretrieve
- Store created orders in memory (map[OrderID]OrderViewRS)
- Return OrderViewRS by OrderID
*/

// --- Minimal XML models (super simplified) ---

type AirShoppingRQ struct {
	XMLName   xml.Name   `xml:"AirShoppingRQ"`
	Version   string     `xml:"Version,attr,omitempty"`
	Document  *Document  `xml:"Document,omitempty"`
	Party     *Party     `xml:"Party,omitempty"`
	CoreQuery CoreQuery  `xml:"CoreQuery"`
	Travelers *Travelers `xml:"Travelers,omitempty"`
}

type OfferPriceRQ struct {
	XMLName      xml.Name  `xml:"OfferPriceRQ"`
	Version      string    `xml:"Version,attr,omitempty"`
	Document     *Document `xml:"Document,omitempty"`
	Party        *Party    `xml:"Party,omitempty"`
	OfferRef     string    `xml:"OfferRef"`
	OfferItemRef string    `xml:"OfferItemRef"`
}

type OrderCreateRQ struct {
	XMLName      xml.Name  `xml:"OrderCreateRQ"`
	Version      string    `xml:"Version,attr,omitempty"`
	Document     *Document `xml:"Document,omitempty"`
	Party        *Party    `xml:"Party,omitempty"`
	OfferRef     string    `xml:"OfferRef"`
	OfferItemRef string    `xml:"OfferItemRef"`
	// super-minimal payment placeholder (optional)
	Payment *Payment `xml:"Payment,omitempty"`
}

type OrderRetrieveRQ struct {
	XMLName  xml.Name  `xml:"OrderRetrieveRQ"`
	Version  string    `xml:"Version,attr,omitempty"`
	Document *Document `xml:"Document,omitempty"`
	Party    *Party    `xml:"Party,omitempty"`
	OrderID  string    `xml:"OrderID"`
}

type Payment struct {
	Amount Amount `xml:"Amount"`
}

type Document struct {
	Name string `xml:"Name,omitempty"`
}

type Party struct {
	Sender *Sender `xml:"Sender,omitempty"`
}

type Sender struct {
	TravelAgencySender *TravelAgencySender `xml:"TravelAgencySender,omitempty"`
}

type TravelAgencySender struct {
	Name string `xml:"Name,omitempty"`
}

type CoreQuery struct {
	OriginDestinations OriginDestinations `xml:"OriginDestinations"`
}

type OriginDestinations struct {
	OD OriginDestination `xml:"OriginDestination"`
}

type OriginDestination struct {
	Departure Departure `xml:"Departure"`
	Arrival   Arrival   `xml:"Arrival"`
}

type Departure struct {
	AirportCode string `xml:"AirportCode"`
	Date        string `xml:"Date"` // YYYY-MM-DD
}

type Arrival struct {
	AirportCode string `xml:"AirportCode"`
}

type Travelers struct {
	Traveler []Traveler `xml:"Traveler"`
}

type Traveler struct {
	AnonymousTraveler AnonymousTraveler `xml:"AnonymousTraveler"`
}

type AnonymousTraveler struct {
	PTC string `xml:"PTC"` // ADT/CHD/INF etc
}

// -------------------- RS MODELS --------------------

type AirShoppingRS struct {
	XMLName       xml.Name    `xml:"AirShoppingRS"`
	CorrelationID string      `xml:"CorrelationID,omitempty"`
	Version       string      `xml:"Version,attr,omitempty"`
	Document      *Document   `xml:"Document,omitempty"`
	Party         *Party      `xml:"Party,omitempty"`
	DataLists     DataLists   `xml:"DataLists"`
	OffersGroup   OffersGroup `xml:"OffersGroup"`
}

type OfferPriceRS struct {
	XMLName       xml.Name    `xml:"OfferPriceRS"`
	CorrelationID string      `xml:"CorrelationID,omitempty"`
	Version       string      `xml:"Version,attr,omitempty"`
	Document      *Document   `xml:"Document,omitempty"`
	Party         *Party      `xml:"Party,omitempty"`
	DataLists     DataLists   `xml:"DataLists"`
	OffersGroup   OffersGroup `xml:"OffersGroup"`
	PricedInd     string      `xml:"PricedInd,attr,omitempty"` // "true"
}

type OrderViewRS struct {
	XMLName       xml.Name  `xml:"OrderViewRS"`
	CorrelationID string    `xml:"CorrelationID,omitempty"`
	Version       string    `xml:"Version,attr,omitempty"`
	Document      *Document `xml:"Document,omitempty"`
	Party         *Party    `xml:"Party,omitempty"`
	DataLists     DataLists `xml:"DataLists"`
	Order         Order     `xml:"Order"`
}

type DataLists struct {
	PassengerList         PassengerList         `xml:"PassengerList"`
	FlightSegmentList     FlightSegmentList     `xml:"FlightSegmentList"`
	PaxJourneyList        PaxJourneyList        `xml:"PaxJourneyList"`
	ServiceDefinitionList ServiceDefinitionList `xml:"ServiceDefinitionList"` // ITERATION 5
}

type PassengerList struct {
	Passengers []Passenger `xml:"Passenger"`
}

type Passenger struct {
	PassengerID string `xml:"PassengerID,attr"`
	PTC         string `xml:"PTC"`
}

type FlightSegmentList struct {
	Segments []FlightSegment `xml:"FlightSegment"`
}

type FlightSegment struct {
	SegmentKey       string           `xml:"SegmentKey,attr"`
	Departure        SegmentDeparture `xml:"Departure"`
	Arrival          SegmentArrival   `xml:"Arrival"`
	MarketingCarrier MarketingCarrier `xml:"MarketingCarrier"`
	FlightNumber     string           `xml:"FlightNumber"`
}

type SegmentDeparture struct {
	AirportCode string `xml:"AirportCode"`
	Date        string `xml:"Date"`
}

type SegmentArrival struct {
	AirportCode string `xml:"AirportCode"`
}

type MarketingCarrier struct {
	AirlineID string `xml:"AirlineID"`
}

type PaxJourneyList struct {
	Journeys []PaxJourney `xml:"PaxJourney"`
}

type PaxJourney struct {
	JourneyKey    string `xml:"JourneyKey,attr"`
	PaxSegmentRef string `xml:"PaxSegmentRef"`
}

type OffersGroup struct {
	CarrierOffers CarrierOffers `xml:"CarrierOffers"`
}

type CarrierOffers struct {
	Offer Offer `xml:"Offer"`
}

type Offer struct {
	OfferID    string     `xml:"OfferID"`
	OfferItems OfferItems `xml:"OfferItems"`
}

type OfferItems struct {
	Items []OfferItem `xml:"OfferItem"`
}

type OfferItem struct {
	OfferItemID   string      `xml:"OfferItemID"`
	JourneyRef    string      `xml:"JourneyRef"`
	PassengerRefs string      `xml:"PassengerRefs"` // space-separated IDs: "PAX1 PAX2"
	PriceDetail   PriceDetail `xml:"PriceDetail"`
	Services      Services    `xml:"Services"` // ITERATION 5
}

// ITERATION 4: PriceDetail (simplified)
type PriceDetail struct {
	BaseAmount  Amount `xml:"BaseAmount"`
	Taxes       Taxes  `xml:"Taxes"`
	TotalAmount Amount `xml:"TotalAmount"`
}

type Taxes struct {
	TotalTaxAmount Amount `xml:"TotalTaxAmount"`
}

type Amount struct {
	Code  string `xml:"Code,attr"`
	Value string `xml:",chardata"`
}

// -------------------- ITERATION 5: SERVICES --------------------

type ServiceDefinitionList struct {
	ServiceDefinitions []ServiceDefinition `xml:"ServiceDefinition"`
}

type ServiceDefinition struct {
	ServiceDefinitionID string `xml:"ServiceDefinitionID,attr"`
	Name                string `xml:"Name"`
}

type Services struct {
	Service []Service `xml:"Service"`
}

type Service struct {
	ServiceID            string      `xml:"ServiceID"`
	ServiceDefinitionRef string      `xml:"ServiceDefinitionRef"`
	PassengerRefs        string      `xml:"PassengerRefs,omitempty"`
	PriceDetail          PriceDetail `xml:"PriceDetail"`
}

// -------------------- ITERATION 7: ORDER --------------------

type Order struct {
	OrderID    string      `xml:"OrderID"`
	Status     string      `xml:"Status"` // CREATED
	OrderItems OrderItems2 `xml:"OrderItems"`
	TotalPrice Amount      `xml:"TotalPrice"`
	CreatedAt  string      `xml:"CreatedAt"`
}

type OrderItems2 struct {
	OrderItem []OrderItem `xml:"OrderItem"`
}

type OrderItem struct {
	OrderItemID   string      `xml:"OrderItemID"`
	OfferRef      string      `xml:"OfferRef"`
	OfferItemRef  string      `xml:"OfferItemRef"`
	JourneyRef    string      `xml:"JourneyRef"`
	PassengerRefs string      `xml:"PassengerRefs"`
	PriceDetail   PriceDetail `xml:"PriceDetail"`
	Services      Services    `xml:"Services"`
}

// -------------------- STUB STATE --------------------

type storedOffer struct {
	OfferID     string
	OfferItemID string
	AirShopping AirShoppingRS
}

var (
	stateMu sync.RWMutex
	state   *storedOffer

	ordersMu sync.RWMutex
	orders   = map[string]OrderViewRS{} // OrderID -> OrderViewRS
)

// -------------------- SERVER --------------------

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/ndc/airshopping", airShopping)
	mux.HandleFunc("/ndc/offerprice", offerPrice)
	mux.HandleFunc("/ndc/ordercreate", orderCreate)
	mux.HandleFunc("/ndc/orderretrieve", orderRetrieve)

	log.Println("NDC stub listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", withLogging(mux)))
}

// -------------------- HANDLERS --------------------

func airShopping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || strings.TrimSpace(string(bodyBytes)) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var rq AirShoppingRQ
	if err := xml.Unmarshal(bodyBytes, &rq); err != nil {
		http.Error(w, "invalid xml body", http.StatusBadRequest)
		return
	}

	from := strings.TrimSpace(rq.CoreQuery.OriginDestinations.OD.Departure.AirportCode)
	to := strings.TrimSpace(rq.CoreQuery.OriginDestinations.OD.Arrival.AirportCode)
	date := strings.TrimSpace(rq.CoreQuery.OriginDestinations.OD.Departure.Date)
	if from == "" || to == "" || date == "" {
		http.Error(w, "missing required fields: Departure/AirportCode, Arrival/AirportCode, Departure/Date", http.StatusBadRequest)
		return
	}

	// pax list default 1 ADT
	ptcs := []string{"ADT"}
	if rq.Travelers != nil && len(rq.Travelers.Traveler) > 0 {
		ptcs = ptcs[:0]
		for _, t := range rq.Travelers.Traveler {
			p := strings.TrimSpace(strings.ToUpper(t.AnonymousTraveler.PTC))
			if p == "" {
				p = "ADT"
			}
			ptcs = append(ptcs, p)
		}
	}

	// PassengerList
	passengers := make([]Passenger, 0, len(ptcs))
	passengerIDs := make([]string, 0, len(ptcs))
	for i, ptc := range ptcs {
		id := fmt.Sprintf("PAX%d", i+1)
		passengers = append(passengers, Passenger{PassengerID: id, PTC: ptc})
		passengerIDs = append(passengerIDs, id)
	}
	paxRefs := strings.Join(passengerIDs, " ")

	seg := FlightSegment{
		SegmentKey:       "SEG1",
		Departure:        SegmentDeparture{AirportCode: from, Date: date},
		Arrival:          SegmentArrival{AirportCode: to},
		MarketingCarrier: MarketingCarrier{AirlineID: "XX"},
		FlightNumber:     "123",
	}
	journey := PaxJourney{JourneyKey: "J1", PaxSegmentRef: "SEG1"}

	// Flight pricing breakdown
	baseTotal, taxTotal := 0.0, 0.0
	for _, ptc := range ptcs {
		switch ptc {
		case "CHD":
			baseTotal += 130
			taxTotal += 19
		case "INF":
			baseTotal += 10
			taxTotal += 2
		default:
			baseTotal += 170
			taxTotal += 29
		}
	}
	flightTotal := baseTotal + taxTotal
	flightPrice := PriceDetail{
		BaseAmount:  Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", baseTotal)},
		Taxes:       Taxes{TotalTaxAmount: Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", taxTotal)}},
		TotalAmount: Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", flightTotal)},
	}

	// BAG service
	bagBase := 25.0 * float64(len(ptcs))
	bagService := Service{
		ServiceID:            "SRV1",
		ServiceDefinitionRef: "SVC_BAG1",
		PassengerRefs:        paxRefs,
		PriceDetail: PriceDetail{
			BaseAmount:  Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", bagBase)},
			Taxes:       Taxes{TotalTaxAmount: Amount{Code: "EUR", Value: "0.00"}},
			TotalAmount: Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", bagBase)},
		},
	}

	offerID := "OFFER-1"
	offerItemID := "ITEM-1"

	rs := AirShoppingRS{
		CorrelationID: "CORR-" + time.Now().Format("20060102150405"),
		Version:       firstNonEmpty(rq.Version, "21.3"),
		Document:      rq.Document,
		Party:         rq.Party,
		DataLists: DataLists{
			PassengerList:     PassengerList{Passengers: passengers},
			FlightSegmentList: FlightSegmentList{Segments: []FlightSegment{seg}},
			PaxJourneyList:    PaxJourneyList{Journeys: []PaxJourney{journey}},
			ServiceDefinitionList: ServiceDefinitionList{
				ServiceDefinitions: []ServiceDefinition{
					{ServiceDefinitionID: "SVC_BAG1", Name: "1 checked bag"},
				},
			},
		},
		OffersGroup: OffersGroup{
			CarrierOffers: CarrierOffers{
				Offer: Offer{
					OfferID: offerID,
					OfferItems: OfferItems{
						Items: []OfferItem{
							{
								OfferItemID:   offerItemID,
								JourneyRef:    "J1",
								PassengerRefs: paxRefs,
								PriceDetail:   flightPrice,
								Services:      Services{Service: []Service{bagService}},
							},
						},
					},
				},
			},
		},
	}

	// store offer state
	stateMu.Lock()
	state = &storedOffer{OfferID: offerID, OfferItemID: offerItemID, AirShopping: rs}
	stateMu.Unlock()

	writeXML(w, http.StatusOK, rs)
}

func offerPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || strings.TrimSpace(string(bodyBytes)) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	var rq OfferPriceRQ
	if err := xml.Unmarshal(bodyBytes, &rq); err != nil {
		http.Error(w, "invalid xml body", http.StatusBadRequest)
		return
	}

	offerRef := strings.TrimSpace(rq.OfferRef)
	itemRef := strings.TrimSpace(rq.OfferItemRef)
	if offerRef == "" || itemRef == "" {
		http.Error(w, "missing required fields: OfferRef, OfferItemRef", http.StatusBadRequest)
		return
	}

	s, ok := loadStoredOffer(offerRef, itemRef, w)
	if !ok {
		return
	}

	rs := OfferPriceRS{
		CorrelationID: "CORR-" + time.Now().Format("20060102150405"),
		Version:       firstNonEmpty(rq.Version, s.AirShopping.Version),
		Document:      rq.Document,
		Party:         rq.Party,
		DataLists:     s.AirShopping.DataLists,
		OffersGroup:   s.AirShopping.OffersGroup,
		PricedInd:     "true",
	}

	writeXML(w, http.StatusOK, rs)
}

func orderCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || strings.TrimSpace(string(bodyBytes)) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var rq OrderCreateRQ
	if err := xml.Unmarshal(bodyBytes, &rq); err != nil {
		http.Error(w, "invalid xml body", http.StatusBadRequest)
		return
	}

	offerRef := strings.TrimSpace(rq.OfferRef)
	itemRef := strings.TrimSpace(rq.OfferItemRef)
	if offerRef == "" || itemRef == "" {
		http.Error(w, "missing required fields: OfferRef, OfferItemRef", http.StatusBadRequest)
		return
	}

	s, ok := loadStoredOffer(offerRef, itemRef, w)
	if !ok {
		return
	}

	selectedOffer := s.AirShopping.OffersGroup.CarrierOffers.Offer
	var selectedItem OfferItem
	found := false
	for _, it := range selectedOffer.OfferItems.Items {
		if it.OfferItemID == itemRef {
			selectedItem = it
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "offer item not found in stored offer", http.StatusBadRequest)
		return
	}

	total := parseAmount(selectedItem.PriceDetail.TotalAmount.Value)
	for _, svc := range selectedItem.Services.Service {
		total += parseAmount(svc.PriceDetail.TotalAmount.Value)
	}

	orderID := "ORD-" + time.Now().Format("20060102150405")
	rs := OrderViewRS{
		CorrelationID: "CORR-" + time.Now().Format("20060102150405"),
		Version:       firstNonEmpty(rq.Version, s.AirShopping.Version),
		Document:      rq.Document,
		Party:         rq.Party,
		DataLists:     s.AirShopping.DataLists,
		Order: Order{
			OrderID:    orderID,
			Status:     "CREATED",
			CreatedAt:  time.Now().Format(time.RFC3339),
			TotalPrice: Amount{Code: "EUR", Value: fmt.Sprintf("%.2f", total)},
			OrderItems: OrderItems2{
				OrderItem: []OrderItem{
					{
						OrderItemID:   "ORDITEM-1",
						OfferRef:      offerRef,
						OfferItemRef:  itemRef,
						JourneyRef:    selectedItem.JourneyRef,
						PassengerRefs: selectedItem.PassengerRefs,
						PriceDetail:   selectedItem.PriceDetail,
						Services:      selectedItem.Services,
					},
				},
			},
		},
	}

	// ITERATION 8: store order by OrderID
	ordersMu.Lock()
	orders[orderID] = rs
	ordersMu.Unlock()

	writeXML(w, http.StatusOK, rs)
}

func orderRetrieve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || strings.TrimSpace(string(bodyBytes)) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var rq OrderRetrieveRQ
	if err := xml.Unmarshal(bodyBytes, &rq); err != nil {
		http.Error(w, "invalid xml body", http.StatusBadRequest)
		return
	}

	orderID := strings.TrimSpace(rq.OrderID)
	if orderID == "" {
		http.Error(w, "missing required field: OrderID", http.StatusBadRequest)
		return
	}

	ordersMu.RLock()
	rs, ok := orders[orderID]
	ordersMu.RUnlock()

	if !ok {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	// Return a fresh correlation id, but keep order data
	rs.CorrelationID = "CORR-" + time.Now().Format("20060102150405")
	// Optionally override document/party from request (handy for testing)
	if rq.Document != nil {
		rs.Document = rq.Document
	}
	if rq.Party != nil {
		rs.Party = rq.Party
	}
	rs.Version = firstNonEmpty(rq.Version, rs.Version)

	writeXML(w, http.StatusOK, rs)
}

// -------------------- HELPERS --------------------

func loadStoredOffer(offerRef, itemRef string, w http.ResponseWriter) (*storedOffer, bool) {
	stateMu.RLock()
	s := state
	stateMu.RUnlock()

	if s == nil {
		http.Error(w, "no offer in memory: call AirShopping first", http.StatusBadRequest)
		return nil, false
	}
	if offerRef != s.OfferID || itemRef != s.OfferItemID {
		http.Error(w, "offer not found (OfferRef/OfferItemRef mismatch)", http.StatusBadRequest)
		return nil, false
	}
	return s, true
}

func parseAmount(v string) float64 {
	v = strings.TrimSpace(v)
	var f float64
	_, _ = fmt.Sscanf(v, "%f", &f)
	return f
}

func writeXML(w http.ResponseWriter, status int, v any) {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, "cannot build response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

func firstNonEmpty(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
