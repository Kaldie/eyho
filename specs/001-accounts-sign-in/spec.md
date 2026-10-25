# Feature Specification: Accounts & Sign-in

**Feature Branch**: `001-accounts-sign-in`

**Created**: 2026-10-25

**Status**: Draft

**Input**: User description: "Add sign-in / log-in." Scope settled in a Q&A session: Google and
email + password sign-in through a managed provider (Auth0), Apple deferred; one account can both
buy and cook; browsing stays open and signing in is required only to claim or to list; verified
email required before claiming or listing; cook onboarding collects display name, organisation
type, phone number and default pickup address; deleted cooks keep an anonymised verification
history; a mock sign-in server for local development; one-tap seeded accounts in the private
demo.

**Position in the roadmap**: first feature. Everything that acts on behalf of a person builds
on these identities: posting and claiming food (002), the frontend's signed-in states (003) and
the demo's one-tap accounts (004). It has no dependency on later features; the buyer and cook
profile tables it extends already exist in the initial schema (migration 000001).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Visitor browses freely, signs in only to claim (Priority: P1)

A visitor browses listings and opens details without any account. When they press Claim, they
are asked to sign in or create an account (Google or email + password). After signing in they
land back in the same claim, with the same listing and quantity, and continue to payment.

**Why this priority**: claiming is the core transaction and needs a known buyer (order history,
ratings, no-show tracking). Keeping browsing open and resuming the claim afterwards keeps
friction low (Constitution Principle V).

**Independent Test**: As a signed-out visitor, browse, press Claim on a listing, sign up with a
new Google account, and confirm the claim dialog reopens for the same listing and quantity.

**Acceptance Scenarios**:

1. **Given** a signed-out visitor, **When** they browse discovery and listing detail pages,
   **Then** everything is visible and no sign-in is requested.
2. **Given** a signed-out visitor who presses Claim, **When** they finish signing in or signing
   up, **Then** they return to the same listing with the claim dialog open and their chosen
   quantity kept.
3. **Given** a signed-out visitor, **When** they open Orders or the cook area directly, **Then**
   they see a short explanation and a sign-in action instead of data.
4. **Given** a signed-in user, **When** they look at the header, **Then** they see that they are
   signed in and can reach their account and sign out; a signed-out visitor sees "Sign in".

---

### User Story 2 - New user signs up and verifies their email (Priority: P1)

A new user creates an account with Google or with email and password. Google sign-ups are ready
to use immediately. Email sign-ups get a verification email and must confirm it before they can
claim or list; until then they can still browse and see a clear prompt to verify.

**Why this priority**: a verified email is the minimum trust anchor for strangers exchanging
food (Principle I) and the channel for pickup and refund notices.

**Independent Test**: Sign up with email and password, try to claim before verifying (blocked
with a prompt and a resend option), verify via the email link, then claim successfully.

**Acceptance Scenarios**:

1. **Given** a new email + password sign-up, **When** the user tries to claim or publish before
   verifying, **Then** the action is blocked with a message and a "resend verification email"
   action.
2. **Given** a user who has clicked the verification link, **When** they return to the site,
   **Then** claiming and listing are unlocked without signing in again.
3. **Given** a Google sign-up, **When** the account is created, **Then** the email counts as
   verified.
4. **Given** someone who signed up with email + password, **When** they later choose "Continue
   with Google" using the same email address, **Then** they reach the same account, not a new
   one.
5. **Given** a user who forgot their password, **When** they request a reset, **Then** they can
   set a new password via an emailed link.

---

### User Story 3 - Buyer becomes a cook (Priority: P1)

A signed-in user who wants to list surplus food starts cook onboarding from the cook area. They
provide a display name, whether they are a non-profit or professional, a phone number and a
default pickup address. They then become a cook (starting as "unverified", per Principle I) and
can create listings, while still being able to claim food as a buyer from the same account.

**Why this priority**: without cooks there is nothing to claim. One account for both roles
matches how neighbours actually behave.

**Independent Test**: Sign in as a buyer-only account, complete cook onboarding, publish a
listing, then claim a different cook's listing from the same account.

**Acceptance Scenarios**:

1. **Given** a signed-in, verified user without a cook profile, **When** they open the cook area,
   **Then** they see cook onboarding asking for display name, organisation type, phone number and
   default pickup address.
2. **Given** onboarding is completed, **When** the user creates a listing, **Then** the pickup
   address defaults to their saved address and can be changed per listing.
3. **Given** a user with a cook profile, **When** they browse and claim, **Then** they can claim
   other cooks' listings like any buyer, but cannot claim their own listings.
4. **Given** onboarding with a missing required field, **When** they submit, **Then** the missing
   field is pointed out and nothing is saved.

---

### User Story 4 - User deletes their account (Priority: P2)

A user can delete their account from their account settings. Their personal data (name, email,
phone, addresses) is removed. If they were a cook, their verification history and ratings stay
in anonymised form so the platform's trust record remains intact (Principle I).

**Why this priority**: required for GDPR in the Netherlands before real people use the service,
but not needed for the first demo walkthroughs.

**Independent Test**: Delete a cook account that has a verification history; confirm the user
can no longer sign in, no personal data remains, and the history rows still exist under an
anonymised cook.

**Acceptance Scenarios**:

1. **Given** a signed-in user, **When** they confirm account deletion, **Then** they are signed
   out, can no longer sign in with that identity, and their personal data is erased.
2. **Given** a cook with verification history and ratings, **When** they delete their account,
   **Then** the history and ratings remain, linked to an anonymised cook shown as "Former cook".
3. **Given** a user with claims still awaiting pickup, or active listings that have claims,
   **When** they try to delete their account, **Then** deletion is blocked with an explanation of
   what must finish first.
4. **Given** a user who deleted their account, **When** they sign up again with the same email,
   **Then** they get a new account with no link to the old history.

---

### User Story 5 - Demo and local sign-in without real accounts (Priority: P2)

In the private demo (feature 004) and in local development, people can sign in with one tap as
a ready-made demo buyer or demo cook, so walkthroughs don't need anyone to create accounts.
Local development uses a mock sign-in server running in Docker Compose, so no provider account
or internet connection is needed.

**Why this priority**: keeps the demo and local setup frictionless, and keeps the constitution's
"`docker compose up` with no extra setup" rule intact.

**Independent Test**: Run `docker compose up` with no provider credentials, sign in as the demo
cook with one tap, publish a listing; then sign in as the demo buyer and claim it.

**Acceptance Scenarios**:

1. **Given** the demo or local environment, **When** a visitor opens sign-in, **Then** they see
   one-tap options for at least a demo buyer, a verified demo cook and an unverified demo cook,
   in addition to normal sign-in.
2. **Given** local development, **When** the stack starts, **Then** sign-in works through a mock
   sign-in service in Docker Compose with no provider keys.
3. **Given** any environment other than demo or local, **When** the application starts with
   one-tap demo sign-in or the mock sign-in service enabled, **Then** it refuses to start.

---

### Edge Cases

- A Google account and an email + password account share an email address: they MUST resolve to
  one account (only when the email is verified on both sides).
- A user signs in, then the provider is unreachable: browsing keeps working; actions needing
  sign-in show a clear "sign-in is temporarily unavailable" message.
- A session expires during the claim flow: the user is asked to sign in again and returned to
  the same step, without losing the reservation if it is still within its payment timeout.
- A cook changes their default pickup address: existing listings keep their own addresses.
- A cook demoted to unverified (Principle I) is unaffected by this feature beyond the label; a
  user's verification status as a cook is unrelated to their email verification, and the UI MUST
  NOT conflate the two.
- Someone tries to claim their own listing: blocked with an explanation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to sign up and sign in with Google, or with email and password.
  Sign-in with Apple is out of scope for this feature.
- **FR-002**: Authentication MUST be handled by a managed identity provider; the platform MUST NOT
  store passwords itself.
- **FR-003**: Browsing discovery and listing details MUST NOT require signing in.
- **FR-004**: Claiming, rating, viewing order history, cook onboarding, creating and managing
  listings, and account settings MUST require a signed-in user.
- **FR-005**: When a signed-out visitor starts an action that requires sign-in, the system MUST
  return them to that action after sign-in, preserving the listing and chosen quantity.
- **FR-006**: Email + password accounts MUST verify their email before claiming or publishing a
  listing; users MUST be able to resend the verification email. Google accounts count as
  verified.
- **FR-007**: Accounts using the same verified email address via different sign-in methods MUST
  resolve to a single account.
- **FR-008**: Users MUST be able to reset a forgotten password by email.
- **FR-009**: One account MUST be able to act as both buyer and cook. Becoming a cook MUST
  require completing cook onboarding with display name, organisation type (non-profit or
  professional), phone number and default pickup address. Phone numbers are stored, not
  verified.
- **FR-010**: A new cook MUST start as "unverified" (Constitution Principle I); onboarding MUST NOT
  imply otherwise.
- **FR-011**: Users MUST NOT be able to claim their own listings.
- **FR-012**: The backend MUST verify the identity of every request to an endpoint that requires
  sign-in and MUST derive the acting buyer or cook from that identity, never from client-supplied
  IDs.
- **FR-013**: Users MUST be able to delete their account themselves. Deletion MUST erase personal
  data (name, email, phone, addresses, provider identity) and MUST keep the cook's verification
  history and ratings in anonymised form. Deletion MUST be blocked while the user has claims
  awaiting pickup or active listings with claims.
- **FR-014**: The header MUST show sign-in state: "Sign in" when signed out; the user's name and a
  menu with account settings and sign out when signed in.
- **FR-015**: Local development MUST use a mock sign-in service in Docker Compose that needs no
  provider account or internet connection (Constitution, Local Development Constraint).
- **FR-016**: The demo and local environments MUST offer one-tap sign-in as seeded accounts: at
  least a buyer, a verified cook and an unverified cook. These seeded accounts MUST be part of
  the demo data set reset on every deploy (feature 004, FR-010).
- **FR-017**: The application MUST refuse to start if one-tap demo sign-in or the mock sign-in
  service is enabled in any environment other than demo or local.

### Key Entities

- **Account**: one person's identity on eyho. Attributes: provider identity, email, email
  verified flag, display name, created/deleted timestamps. Has at most one Buyer Profile and at
  most one Cook Profile.
- **Buyer Profile** (table from migration 000001, used by feature 002): now owned by an Account;
  created on first sign-in.
- **Cook Profile** (table from migration 000001, used by feature 002): now owned by an Account; created by cook onboarding. Adds phone
  number and default pickup address. On account deletion, personal fields are erased and the
  profile is marked anonymised ("Former cook") while its verification history and ratings stay.
- **Demo Account**: a seeded Account available for one-tap sign-in in demo and local only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new visitor can go from pressing Claim to being back in the claim dialog, signed
  up, in under 60 seconds with Google.
- **SC-002**: A signed-in user can complete cook onboarding in under 2 minutes.
- **SC-003**: 100% of requests to sign-in-only endpoints without a valid identity are refused.
- **SC-004**: After account deletion, no personal data of that user is retrievable through the
  product or its database, while 100% of their verification history rows remain.
- **SC-005**: A fresh checkout of the repository can sign in locally with `docker compose up`
  and no provider credentials.
- **SC-006**: Authentication costs €0 per month at pilot scale.

## Assumptions

- **Provider (decided 2026-10-25)**: Auth0, using its free tier. Chosen over Firebase Auth and
  Clerk by the maintainer. The plan phase MUST confirm current free-tier limits and EU data
  residency options.
- **Apple sign-in** is deferred because it needs a paid Apple Developer account (about €99 per
  year); adding it later MUST NOT require changes beyond provider configuration and one button.
- **Local development** uses a standards-based mock sign-in server container; the backend talks
  to it exactly as it talks to Auth0, so no code paths differ between local and demo beyond
  configuration.
- **Sessions**: users stay signed in across visits for a reasonable period (for example 30 days)
  unless they sign out; exact duration is a plan decision.
- **Notifications** (pickup confirmations, refunds) use the account's verified email; the
  notification channel itself remains a feature 002 implementation detail.
- **Deletion while active**: blocking deletion until pickups and listings finish avoids defining
  new refund rules; revisit if users find it frustrating.
- **Out of scope**: Apple sign-in, phone/SMS verification, two-factor authentication, admin or
  moderator accounts, organisation accounts with multiple members, identity or food-safety
  registration checks.
