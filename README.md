# 🔍 Cracked Dev-o-Meter

A humorous yet sophisticated tool that evaluates developer "crackedness" by analyzing their GitHub repositories and social presence. Built with modern web technologies and advanced statistical analysis.

![Cracked Dev-o-Meter](https://img.shields.io/badge/crackedness-analyzer-blue?style=for-the-badge)
![SolidJS](https://img.shields.io/badge/frontend-SolidJS-2c4f7c?style=flat-square)
![Go](https://img.shields.io/badge/backend-Go-00ADD8?style=flat-square)
![Gin](https://img.shields.io/badge/framework-Gin-008000?style=flat-square)

## 🚀 Features

- **🎨 Beautiful Glassmorphic UI** - Modern, zen-like interface with smooth animations
- **📊 Advanced Scoring Algorithm** - Bayesian analysis with 10+ postulates and anti-gaming measures
- **🔍 GitHub + X Integration** - Combined analysis of development activity AND social presence
- **📈 Multi-dimensional Scoring** - Shipping velocity, code quality, influence, complexity, collaboration, and more
- **🎯 Explainable Results** - Detailed breakdown of what makes a developer "cracked"
- **⚡ Real-time Updates** - Live analysis with animated SVG meter
- **🐳 Docker Support** - Complete containerization with docker-compose
- **🔒 Security First** - Comprehensive security scanning and best practices
- **🔄 CI/CD Ready** - Full GitHub Actions workflows for testing and deployment
- **📱 Responsive Design** - Works perfectly on desktop and mobile

## 🏗️ Architecture

### Frontend (SolidJS + Vite)

- **SolidJS 1.8.22** - Reactive framework with excellent performance
- **Vite 6.0** - Lightning-fast build tool and dev server
- **TailwindCSS + DaisyUI** - Modern utility-first CSS framework
- **TypeScript** - Full type safety and excellent DX

### Backend (Go + Gin)

- **Gin Framework** - High-performance HTTP web framework
- **Hexagonal Architecture** - Clean separation with ports & adapters
- **Advanced Statistics** - Robust normalization, Bayesian aggregation
- **GitHub GraphQL API** - Real-time repository and user data fetching
- **X (Twitter) API v2** - Social media presence and engagement analysis
- **Combined Analysis** - Unified scoring from multiple data sources
- **Graceful Fallbacks** - Continues analysis even when APIs are unavailable

### Scoring Algorithm

- **10 Formal Postulates** - Mathematical foundation for developer evaluation
- **Bayesian Aggregation** - Probabilistic combination of multiple signals
- **Anti-gaming Measures** - Robust normalization and duplicate detection
- **7 Categories**: Shipping, Quality, Influence, Complexity, Collaboration, Reliability, Novelty
- **Combined GitHub + X Analysis**: Enhanced scoring using both development activity and social media presence

## 🔍 Combined GitHub + X Analysis

The system now supports analyzing developers using **both** their GitHub and X (Twitter) presence for more comprehensive evaluation:

### Input Formats

| Format                | Example                      | Description                                      |
| --------------------- | ---------------------------- | ------------------------------------------------ |
| **GitHub Username**   | `torvalds`                   | Analyze GitHub activity only                     |
| **GitHub Repository** | `facebook/react`             | Analyze specific repository                      |
| **X Username**        | `@elonmusk`                  | Analyze Twitter presence only                    |
| **Combined Analysis** | `github:torvalds x:elonmusk` | **BEST**: Full analysis combining both platforms |

### What Combined Analysis Provides

- **Enhanced Influence Scoring**: GitHub stars/forks + Twitter followers/engagement
- **Social Sentiment Analysis**: Twitter content sentiment and engagement patterns
- **Cross-Platform Validation**: Verifies developer presence across platforms
- **Comprehensive Profile**: Complete view of technical and social influence
- **Higher Coverage Score**: More data sources = more confident analysis

### Fallback Behavior

The system gracefully handles API failures:

- ✅ **GitHub fails, X succeeds** → Continues with X-only analysis
- ✅ **X fails, GitHub succeeds** → Continues with GitHub-only analysis
- ✅ **Both fail** → Returns helpful error message
- ✅ **No tokens configured** → Logs warning, continues with available data

## 🛠️ Installation & Setup

### Prerequisites

- **Node.js 18+** with pnpm (for development)
- **Go 1.21+** (for development)
- **Docker & Docker Compose** (recommended for easy setup)
- **GitHub Personal Access Token** (optional, for higher rate limits)
- **X (Twitter) Bearer Token** (optional, for combined GitHub+X analysis)

### Quick Start

1. **Clone the repository**

   ```bash
   git clone https://github.com/your-org/cracked-dev-o-meter.git
   cd cracked-dev-o-meter
   ```

2. **Set up environment variables**
   ```bash
   cp env.example .env
   # Edit .env with your tokens (optional but recommended for full functionality)
   ```

#### 🚀 Docker Setup (Recommended)

3. **Start the complete application**

   ```bash
   docker-compose up --build
   ```

   Access the application at: **http://localhost**

   This starts:

   - **Backend API** (Go/Gin) on port 8080
   - **Frontend** (SolidJS) on port 3000
   - **Nginx Reverse Proxy** on port 80
   - All services communicate seamlessly

4. **View logs**
   ```bash
   docker-compose logs -f [service-name]
   ```

#### 🛠️ Local Development Setup

3. **Start the backend**

   ```bash
   cd backend
   go mod tidy
   export GITHUB_TOKEN=your_github_token_here      # Optional
   export X_BEARER_TOKEN=your_twitter_token_here   # Optional for combined analysis
   go run ./cmd/server
   ```

   Backend API available at: http://localhost:8080

4. **Start the frontend** (new terminal)

   ```bash
   cd frontend
   pnpm install
   pnpm run dev
   ```

   Frontend available at: http://localhost:5173

5. **Open your browser**
   - Full app via Docker: http://localhost
   - Frontend (local): http://localhost:5173
   - Backend API: http://localhost:8080

## 🔄 CI/CD & Quality Assurance

The project includes comprehensive CI/CD pipelines:

### GitHub Actions Workflows

- **Backend CI** (`backend-ci.yml`): Go testing, linting, security scanning, building
- **Frontend CI** (`frontend-ci.yml`): Node.js testing, building, security scanning

### Quality Gates

- **Testing**: 70+ tests covering backend, frontend, and integration scenarios
- **Linting**: ESLint (frontend) + golangci-lint (backend)
- **Security**: Gosec, Trivy vulnerability scanning
- **Coverage**: Codecov integration for coverage tracking

### Running Locally

```bash
# Backend tests
cd backend && go test ./... -v

# Frontend tests
cd frontend && pnpm test

# Full test suite
docker-compose exec backend go test ./... -v
docker-compose exec frontend pnpm test
```

## 📡 API Usage

### Analyze Endpoint

**POST** `/analyze` or `/api/analyze`

**Request Body:**

```json
{
  "input": "github:torvalds x:elonmusk"
}
```

**Response:**

```json
{
  "score": 95,
  "confidence": 0.89,
  "posterior": 0.91,
  "breakdown": {
    "shipping": 92.3,
    "quality": 88.7,
    "influence": 97.1,
    "complexity": 85.4,
    "collaboration": 91.2,
    "reliability": 93.8,
    "novelty": 89.6
  },
  "contributors": [...]
}
```

### Health Check

**GET** `/health` or `/api/health`

**Response:**

```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0"
}
```

### Development Commands

#### Frontend

```bash
cd frontend
pnpm install          # Install dependencies
pnpm run dev         # Start dev server
pnpm run build       # Production build
pnpm run preview     # Preview production build
pnpm run type-check  # TypeScript checking
pnpm run lint        # ESLint checking
pnpm run format      # Prettier formatting
```

#### Backend

```bash
cd backend
go mod tidy           # Download dependencies
go run ./cmd/server   # Start development server
go build ./cmd/server # Build binary
go test ./...         # Run all tests
```

## 📖 Usage

1. **Enter a GitHub repository** (e.g., `github.com/microsoft/vscode`)
2. **Or enter a username** (e.g., `torvalds`)
3. **Click Analyze** to see the crackedness score
4. **View detailed breakdown** of contributing factors

### Example Analysis

```json
{
  "score": 83,
  "confidence": 0.78,
  "posterior": 0.83,
  "contributors": [
    { "name": "shipping.star_velocity", "contribution": 0.42 },
    { "name": "quality.review_depth", "contribution": 0.31 }
  ],
  "breakdown": {
    "shipping": 0.95,
    "quality": 0.76,
    "influence": 0.81,
    "complexity": 0.64,
    "collaboration": 0.58,
    "reliability": 0.61,
    "novelty": 0.43
  }
}
```

## 🔬 Algorithm Details

### Core Postulates

- **P1**: Recency Monotonicity - Recent activity matters more
- **P2**: Diminishing Returns - Avoid over-rewarding repetitive actions
- **P3**: Quality-over-Quantity - Peer validation beats raw metrics
- **P4**: Difficulty/Complexity Reward - Harder work = higher scores
- **P5**: Collaboration Balance - Authorship + meaningful reviews
- **P6**: Reliability - CI success and low revert rates
- **P7**: Influence & Reach - Network effects and downstream usage
- **P8**: Anti-gaming - Robust normalization prevents manipulation
- **P9**: Multi-modal Synergy - Combine signals probabilistically
- **P10**: Explainability - Decompose scores into understandable factors

### Mathematical Foundation

- **Decay Functions**: `w(t) = exp(-(T-t)/τ)` with dual horizons
- **Robust Z-scores**: `asinh((x - median)/MAD)` with clipping
- **Bayesian Aggregation**: `L = ∑w_k * ell_k`, `p = sigmoid(L)`

## 🧪 Testing

### Backend Tests

```bash
cd backend
go test ./internal/analysis -v  # Core algorithm tests
go test ./internal/adapters -v  # API adapter tests
go test ./... -cover           # Full test coverage
```

### Frontend Tests

```bash
cd frontend
# Tests coming soon - focusing on MVP functionality first
```

## 🚀 Deployment

### Docker Deployment

```bash
# Build and run with Docker
docker-compose up -d
```

### Manual Deployment

```bash
# Backend
cd backend
go build -o cracked-meter ./cmd/server
./cracked-meter

# Frontend
cd frontend
pnpm run build
# Serve dist/ folder with any static server
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes with proper tests
4. Run the full test suite: `go test ./... && cd frontend && pnpm run type-check`
5. Commit with conventional commits: `git commit -m "feat: add amazing feature"`
6. Push to your branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **SolidJS** - For the incredible reactive framework
- **Go + Gin** - For blazing-fast backend development
- **GitHub API** - For making developer data accessible
- **TailwindCSS** - For beautiful, maintainable styling

## 🎯 Roadmap

- [ ] X/Twitter integration for social signals
- [ ] Advanced ML models for better predictions
- [ ] Real-time collaboration features
- [ ] Mobile app development
- [ ] Integration with other developer platforms
- [ ] Advanced visualization and analytics

---

**Built with ❤️ for the developer community**

_Remember: Being "cracked" is a compliment in the world of software development!_ 🚀
