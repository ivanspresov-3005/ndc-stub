# NDC-Based Dynamic Offer Simulation  
## Quantifying Revenue Uplift Through Offer Personalization (MBA Research)

This repository contains an **NDC-first simulation environment** developed as part of an MBA research project on **airline dynamic retailing**.
The project intentionally **starts from a functional NDC stub** (AirShopping → OfferPrice → OrderCreate) and incrementally builds toward **financial modeling of revenue uplift** enabled by dynamic offer generation, bundling, and personalization.
Rather than treating NDC as a messaging standard, this research uses it as a **retail abstraction layer** to connect offer construction mechanics with measurable commercial outcomes.

---

## Research objective

**Primary question**

> How can dynamic, personalized offers—implemented through NDC-style Offer/Order flows—be translated into quantifiable revenue uplift at route and passenger level?

**Secondary questions**
- Which offer levers (price, bundle composition, ancillaries) drive the largest uplift?
- How does personalization change willingness-to-pay capture compared to classical fare families?
- How can NDC-based retail logic be used as an experimental platform for financial modeling?

---

## Why start from an NDC stub?

Most research discusses NDC conceptually.  
This project starts from the **opposite direction**:

- implement a **minimal but realistic NDC flow**
- treat Offers and Orders as **economic objects**
- layer financial logic on top of technical primitives

This allows:
- controlled experimentation
- reproducible simulations
- direct mapping between **technical decisions** and **financial impact**

---

## Implemented NDC flow (current state)

The stub implements a simplified but structurally correct NDC lifecycle:
AirShoppingRQ
↓
AirShoppingRS (Offers + PriceDetail + Services)
↓
OfferPriceRQ
↓
OfferPriceRS (confirmation / repricing)
↓
OrderCreateRQ
↓
OrderViewRS (Order created)

### Supported concepts
- Offer / OfferItem
- Passenger & Journey references (DataLists)
- PriceDetail (Base / Taxes / Total)
- Ancillary services via ServiceDefinitionList
- Offer confirmation (OfferPrice)
- Order creation (ONE Order mindset)

This forms the **technical backbone** for the financial model.

> The **NDC stub is the foundation**, not a side artifact.

---

## Conceptual mapping: NDC → Financial model

| NDC Concept | Economic Interpretation |
|------------|------------------------|
| Offer | Commercial proposition |
| OfferItem | Per-passenger revenue unit |
| PriceDetail | Price realization structure |
| Service | Ancillary revenue stream |
| OfferPrice | Commitment / price integrity |
| Order | Realized revenue event |

This mapping allows financial analysis **without abandoning NDC semantics**.

---

## Scenarios modeled (planned & in progress)

### Scenario A — Classical RM baseline
- static fare families
- limited bundling
- ancillaries sold post-selection
- price determined primarily by inventory control

### Scenario B — Dynamic offers (non-personalized)
- offers constructed dynamically
- attribute-based bundles
- contextual pricing (route, time, demand)

### Scenario C — Dynamic offers with personalization
- bundle composition adapted to passenger context
- willingness-to-pay signals influence pricing
- higher attachment and conversion expected

The **same NDC stub** is reused across all scenarios; only offer logic changes.

---

## Financial metrics of interest

The modeling layer (to be built on top of the stub) focuses on:

- Revenue per passenger uplift
- Conversion uplift attributable to offer relevance
- Ancillary attachment rate
- Willingness-to-pay capture vs static pricing
- Route-level revenue uplift distribution
- Sensitivity to elasticity and behavioral assumptions

---

## Methodology (high level)

1. Use the NDC stub to generate controlled offers
2. Apply behavioral assumptions (conversion, attachment, WTP)
3. Simulate purchase outcomes via OrderCreate
4. Aggregate Order-level results into route-level metrics
5. Run sensitivity analysis / Monte Carlo simulations

The stub ensures **structural consistency** across all experiments.

