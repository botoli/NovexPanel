# 🚀 NovexPanel

**NovexPanel** — это современная, быстрая и удобная панель управления серверами. Она позволяет осуществлять мониторинг системных ресурсов, управлять процессами, выполнять деплой приложений и работать в полноценном веб-терминале прямо из браузера.

<img width="1916" height="918" alt="изображение" src="https://github.com/user-attachments/assets/f63495de-1010-41ef-b27b-d40333f75e1d" />


## ✨ Ключевые возможности

* **📊 Мониторинг в реальном времени:** Отслеживание нагрузки CPU, использования RAM, диска и сети без задержек.
  
* **💻 Встроенный Web-Терминал:** Полноценный защищенный доступ к командной строке контролируемого сервера через браузер (WebSockets).
  <img width="1917" height="916" alt="изображение" src="https://github.com/user-attachments/assets/a40483f6-35b8-4199-a57f-1c929e96b363" />
* **⚙️ Управление процессами:** Просмотр списка процессов на целевом сервере в реальном времени для выявления узких мест.
  <img width="1918" height="915" alt="изображение" src="https://github.com/user-attachments/assets/5a1a3f3c-34b6-4978-8bdb-fc52db6666cf" />
* **🚀 Менеджер деплоя:** Удобный интерфейс для настройки, развертывания и управления версиями пользовательских приложений.
  <img width="1919" height="916" alt="изображение" src="https://github.com/user-attachments/assets/291c86a6-ab9d-42d4-87ca-6fa9e5d621bd" />
  <img width="1919" height="915" alt="изображение" src="https://github.com/user-attachments/assets/3dec0b13-ecb2-413f-ab56-f572c96065bb" />
* **🔐 Гибкая безопасность:** JWT-авторизация, управление аккаунтами и защищенное общение с серверами на базе изолированных токенов агентов.
  <img width="777" height="410" alt="изображение" src="https://github.com/user-attachments/assets/ce765f53-5183-4b98-8be6-cdb34d1841c0" />


## 🏗 Архитектура

Проект состоит из трех основных частей:
1. **Frontend (Пользовательская панель):** Быстрое SPA-приложение на React + TypeScript, собранное через Vite. Дизайн реализован с помощью модульного SCSS.
2. **Backend (Управляющий узел):** Высокопроизводительное API на базе Go с активным использованием WebSockets для двунаправленной связи (терминалы, метрики) и PostgreSQL для хранения конфигураций.
3. **Agent (Служебный демон):** Легковесная программа на Go, которая устанавливается на целевые серверы. Агент принимает команды от бэкенда, собирает статистику и отдает поток терминала.

flowchart TB
    subgraph Browser["Браузер SPA"]
        FE["React 19 + TypeScript + MobX
            Dev-сервер Vite :5174"]
    end

    subgraph BackendServer["Сервер бэкенда"]
        GB["HTTP-сервер Gin
            Порт :8380"]
        HUB["WebSocket Hub
            /site/ws + /agent/ws"]
        DB[(PostgreSQL / SQLite
            ORM GORM)]
        RATELIMITER["Ограничитель запросов
            In-memory fixed window"]
    end

    subgraph ManagedServer["Управляемый сервер 1..N"]
        AGENT["Бинарный файл агента Go
            WebSocket-клиент + PTY + Docker"]
    end

    FE -->|REST API + WS| GB
    FE -->|WS /site/ws| HUB
    GB --> DB
    HUB -->|WS /agent/ws| AGENT
    GB -->|REST /terminal/:id| AGENT
    AGENT -->|gopsutil| OS["Метрики ОС
        CPU / RAM / Диск / Сеть"]
    AGENT -->|Docker SDK| DOCKER["Docker Engine
        Сборка + Запуск контейнеров"]
    AGENT -->|creack/pty| PTY["PTY Shell
        bash -lc"]


## 🛠 Технологический стек

**Frontend:**
- React / TypeScript
- Различные Store-модули (состояние агентов, деплоев, метрик)
- Интерфейс на SCSS Modules
- Сборка Vite

**Backend & Agent:**
- Go (Golang)
- WebSockets (Gorilla WS или аналоги)
- JWT (JSON Web Tokens)
- PostgreSQL (через docker-compose для базы данных)

## 🚀 Установка и быстрый старт

### Требования
- Node.js 18+
- Go 1.20+
- Docker и Docker Compose (для запуска требуемой инфраструктуры)

### Развертывание Backend-части
1. Перейдите в директорию бэкенда:
   ```bash
   cd backend
   ```
2. Поднимите инфраструктуру (БД):
   ```bash
   docker-compose up -d
   ```
3. Запустите API-сервер:
   ```bash
   go run cmd/server/main.go
   ```
4. *(Для разработки/тестирования)* Запустите локального агента:
   ```bash
   go run cmd/agent/main.go
   ```

### Развертывание Frontend-части
1. Перейдите в директорию фронтенда:
   ```bash
   cd frontend
   ```
2. Установите зависимости:
   ```bash
   npm install
   ```
3. Запустите панель в режиме разработки:
   ```bash
   npm run dev
   ```
4. Откройте в браузере предоставленную ссылку (по умолчанию `http://localhost:5173`).

## 👨‍💻 Структура исходного кода

- `backend/cmd/` — точки входа для Server и Agent.
- `backend/internal/` — бизнес-логика (работа с WS, авторизация, база данных, эндпоинты деплоя).
- `frontend/src/Pages/` — главные экраны приложения (Home, LeftPanel) и страницы сервера (Terminal, Processes, Deploy, Metrics).
- `frontend/src/modals/` — модальные окна (Account, Registration, Login, Confirm).
- `frontend/src/Store/` — управление глобальным состоянием React.

## 📄 Лицензия
Распространяется на условиях лицензии MIT. Подробнее см. файл `LICENSE`.
