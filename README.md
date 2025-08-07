# NomNom-Radar 🍔📡

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/slighter12/NomNom-Radar)
[![Contributions Welcome](https://img.shields.io/badge/contributions-welcome-orange.svg)](https://github.com/slighter12/NomNom-Radar/issues)

Your personal foodie radar! Automatically detects nearby food trucks and sends out a "Nom Nom" alert.

NomNom-Radar is a real-time notification backend service. It utilizes PostgreSQL with PostGIS for high-performance geospatial queries to track mobile food vendors and notify users when deliciousness is near.

## ✨ Core Features

* **Real-time Vendor Tracking**: Allows food truck owners to update their location instantly via a simple API endpoint.
* **Dynamic Geofencing**: Users can define a custom notification radius (e.g., 500 meters, 1 km).
* **High-Performance Geospatial Queries**: Leverages the power of PostGIS (`ST_DWithin`) for efficient, low-latency proximity checks.
* **Push Notification System**: Integrates with services like Firebase Cloud Messaging (FCM) to deliver instant alerts to users' devices.
* **Open & Share-Alike**: Licensed under AGPL-3.0 to encourage community contribution while protecting the project from being used in proprietary, closed-source commercial services.

## 🛠️ Tech Stack

* **Backend**: **Go** with a standard library or a framework like **Gin / Echo**
* **Database**: PostgreSQL + PostGIS extension for geospatial data
* **Database Driver/ORM**: **GORM / sqlx**
* **Push Notifications**: Firebase Cloud Messaging (FCM)
* **Containerization**: Docker & Docker Compose

## 🚀 Getting Started

Follow these instructions to get a local copy up and running for development and testing purposes.

### Prerequisites

* [Go](https://go.dev/doc/install) (e.g., v1.21 or later)
* [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
* A [Firebase](https://firebase.google.com/) project for push notifications.

### Installation & Setup

1.  **Clone the repository:**
    ```bash
    git clone [https://github.com/YOUR_USERNAME/NomNom-Radar.git](https://github.com/YOUR_USERNAME/NomNom-Radar.git)
    cd NomNom-Radar
    ```

## 🤔 How It Works

1.  **Vendor Pings Location**: A food truck owner sends a `POST` request with their `vendorId` and current coordinates (`latitude`, `longitude`) to the `/api/vendors/location` endpoint.
2.  **Location is Stored**: The Go service updates the vendor's location in the PostGIS database. The location is stored as a `GEOGRAPHY` or `GEOMETRY` type using a Go database driver.
3.  **Proximity Check Job**: A background Goroutine or scheduled job runs periodically (e.g., every minute).
4.  **Geospatial Query**: The worker queries the database, asking: "For each user, are there any vendors within their specified notification radius?" This is efficiently handled by the PostGIS `ST_DWithin` function.
5.  **Notification Sent**: If a match is found, the service triggers a push notification via FCM to the relevant user's device, letting them know a favorite food truck is nearby.

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/YOUR_USERNAME/NomNom-Radar/issues).

Please adhere to this project's `code of conduct`.

## 📄 License

This project, `NomNom-Radar`, is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

In simple terms, this means:

* **✓ Freedom to Use**: You are free to run, study, share, and modify the software.
* **🔗 Share Alike**: If you modify this project's code and make it available as a public network service (e.g., a website or API), you **must** also release your modified source code to all users of that service under the same AGPL-3.0 license.

The choice of AGPL-3.0 is intended to foster community collaboration while preventing this project from being used to create proprietary, closed-source commercial services. We welcome all contributions, but please ensure you understand and agree to the terms of this license.

See the [LICENSE](LICENSE) file for the full legal text.

---
*Copyright (c) 2025, [Your Name or Company Name]*
