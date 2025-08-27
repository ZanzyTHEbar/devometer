# 🔍 Cracked Dev-o-Meter

A humorous yet sophisticated tool that evaluates developer "crackedness" by analyzing their GitHub repositories and social presence. Built with modern web technologies and advanced statistical analysis.

![Cracked Dev-o-Meter](https://img.shields.io/badge/crackedness-analyzer-blue?style=for-the-badge)
![SolidJS](https://img.shields.io/badge/frontend-SolidJS-2c4f7c?style=flat-square)
![Go](https://img.shields.io/badge/backend-Go-00ADD8?style=flat-square)
![Gin](https://img.shields.io/badge/framework-Gin-008000?style=flat-square)

## 🚀 Features

- **🎨 Beautiful Glassmorphic UI** - Modern, zen-like interface with smooth animations
- **📊 Advanced Scoring Algorithm** - Bayesian analysis with 10+ postulates and anti-gaming measures
- **🔍 GitHub Integration** - Real-time analysis of repositories, commits, and contributions
- **📈 Multi-dimensional Scoring** - Shipping velocity, code quality, influence, complexity, and more
- **🎯 Explainable Results** - Detailed breakdown of what makes a developer "cracked"
- **⚡ Real-time Updates** - Live analysis with animated SVG meter

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
- **GitHub GraphQL API** - Real-time data fetching with proper rate limiting

### Scoring Algorithm
- **10 Formal Postulates** - Mathematical foundation for developer evaluation
- **Bayesian Aggregation** - Probabilistic combination of multiple signals
- **Anti-gaming Measures** - Robust normalization and duplicate detection
- **7 Categories**: Shipping, Quality, Influence, Complexity, Collaboration, Reliability, Novelty

## 🛠️ Installation & Setup

### Prerequisites
- **Node.js 18+** with pnpm
- **Go 1.21+**
- **GitHub Personal Access Token** (optional, for higher rate limits)

### Quick Start

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-org/cracked-dev-o-meter.git
   cd cracked-dev-o-meter
   ```

2. **Setup Backend**
   ```bash
   cd backend
   go mod tidy
   export GITHUB_TOKEN=your_github_token_here  # Optional
   go run ./cmd/server
   ```

3. **Setup Frontend**
   ```bash
   cd frontend
   pnpm install
   pnpm run dev
   ```

4. **Open your browser**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080

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
    {"name": "shipping.star_velocity", "contribution": 0.42},
    {"name": "quality.review_depth", "contribution": 0.31}
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

*Remember: Being "cracked" is a compliment in the world of software development!* 🚀
