package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// DebugServer представляет HTTP сервер для отладки и профилирования.
type DebugServer struct {
	server *http.Server
	logger *slog.Logger
}

// DebugServerDeps содержит зависимости для инициализации Debug сервера.
type DebugServerDeps struct {
	Addr   string
	Logger *slog.Logger
}

// NewDebugServer создает и настраивает debug сервер с pprof endpoints.
func NewDebugServer(deps *DebugServerDeps) *DebugServer {
	router := chi.NewRouter()

	// Middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)

	debugSrv := &DebugServer{
		logger: deps.Logger,
	}

	// Главная страница с HTML интерфейсом
	router.Get("/", debugSrv.handleDebugIndex)

	// pprof endpoints
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// pprof handlers для различных профилей
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	router.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

	server := &http.Server{
		Addr:              deps.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	debugSrv.server = server
	return debugSrv
}

// Start запускает Debug HTTP сервер.
func (s *DebugServer) Start() error {
	s.logger.Info("Debug server starting", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop останавливает Debug HTTP сервер с graceful shutdown.
func (s *DebugServer) Stop(ctx context.Context) error {
	s.logger.Info("Debug server stopping...")
	return s.server.Shutdown(ctx)
}

// handleDebugIndex отображает HTML страницу с интерфейсом для доступа к pprof endpoints.
func (s *DebugServer) handleDebugIndex(w http.ResponseWriter, _ *http.Request) {
	html := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OtusMS Debug & Profiling</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
            line-height: 1.6;
            padding: 20px;
            min-height: 100vh;
        }
        
        .container {
            max-width: 1000px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            overflow: hidden;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 40px;
            text-align: center;
        }
        
        .header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            font-weight: 600;
        }
        
        .header p {
            font-size: 1.1em;
            opacity: 0.9;
        }
        
        .content {
            padding: 40px;
        }
        
        .warning {
            background: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 15px 20px;
            margin-bottom: 30px;
            border-radius: 4px;
        }
        
        .warning strong {
            color: #856404;
            display: block;
            margin-bottom: 5px;
        }
        
        .section {
            margin-bottom: 40px;
        }
        
        .section h2 {
            color: #667eea;
            font-size: 1.8em;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #e0e0e0;
        }
        
        .endpoints {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        
        .endpoint {
            background: #f8f9fa;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            padding: 20px;
            transition: all 0.3s ease;
        }
        
        .endpoint:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
            border-color: #667eea;
        }
        
        .endpoint h3 {
            color: #333;
            font-size: 1.2em;
            margin-bottom: 10px;
        }
        
        .endpoint p {
            color: #666;
            font-size: 0.95em;
            margin-bottom: 15px;
        }
        
        .endpoint a {
            display: inline-block;
            background: #667eea;
            color: white;
            text-decoration: none;
            padding: 10px 20px;
            border-radius: 6px;
            font-weight: 500;
            transition: background 0.3s ease;
        }
        
        .endpoint a:hover {
            background: #5568d3;
        }
        
        .code-block {
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 20px;
            border-radius: 8px;
            overflow-x: auto;
            font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
            font-size: 0.9em;
            line-height: 1.5;
            margin-bottom: 15px;
        }
        
        .code-block code {
            color: #f8f8f2;
        }
        
        .info {
            background: #e3f2fd;
            border-left: 4px solid #2196f3;
            padding: 15px 20px;
            margin-bottom: 20px;
            border-radius: 4px;
        }
        
        .info strong {
            color: #1565c0;
            display: block;
            margin-bottom: 5px;
        }
        
        ul {
            margin-left: 20px;
            margin-bottom: 15px;
        }
        
        li {
            margin-bottom: 8px;
        }
        
        .footer {
            background: #f8f9fa;
            padding: 20px 40px;
            text-align: center;
            color: #666;
            border-top: 1px solid #e0e0e0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔧 OtusMS Debug & Profiling</h1>
            <p>Инструменты для отладки и анализа производительности</p>
        </div>
        
        <div class="content">
            <div class="warning">
                <strong>⚠️ Предупреждение безопасности</strong>
                Этот интерфейс предоставляет доступ к внутренней информации приложения. Убедитесь, что доступ защищен!
            </div>
            
            <div class="section">
                <h2>📊 Профили производительности</h2>
                <div class="endpoints">
                    <div class="endpoint">
                        <h3>🧠 Heap Profile</h3>
                        <p>Анализ использования памяти и выделений объектов в куче</p>
                        <a href="/debug/pprof/heap" target="_blank">Открыть heap</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>⚡ CPU Profile</h3>
                        <p>Профилирование использования CPU (по умолчанию 30 сек)</p>
                        <a href="/debug/pprof/profile" target="_blank">Открыть profile</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🔄 Goroutines</h3>
                        <p>Список всех активных горутин и их стеки вызовов</p>
                        <a href="/debug/pprof/goroutine" target="_blank">Открыть goroutine</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🔒 Block Profile</h3>
                        <p>Блокировки синхронизации (mutex, channel)</p>
                        <a href="/debug/pprof/block" target="_blank">Открыть block</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🧵 Thread Create</h3>
                        <p>Создание новых OS потоков</p>
                        <a href="/debug/pprof/threadcreate" target="_blank">Открыть threadcreate</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🔐 Mutex Profile</h3>
                        <p>Конфликты при захвате мьютексов</p>
                        <a href="/debug/pprof/mutex" target="_blank">Открыть mutex</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>📦 Allocs</h3>
                        <p>Все выделения памяти (включая освобожденные)</p>
                        <a href="/debug/pprof/allocs" target="_blank">Открыть allocs</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🔍 Index</h3>
                        <p>Главная страница pprof со всеми доступными профилями</p>
                        <a href="/debug/pprof/" target="_blank">Открыть index</a>
                    </div>
                </div>
            </div>
            
            <div class="section">
                <h2>🛠️ Дополнительные endpoints</h2>
                <div class="endpoints">
                    <div class="endpoint">
                        <h3>📋 Command Line</h3>
                        <p>Аргументы командной строки приложения</p>
                        <a href="/debug/pprof/cmdline" target="_blank">Открыть cmdline</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>🔎 Symbol</h3>
                        <p>Lookup символов по адресу программы</p>
                        <a href="/debug/pprof/symbol" target="_blank">Открыть symbol</a>
                    </div>
                    
                    <div class="endpoint">
                        <h3>📈 Trace</h3>
                        <p>Execution trace для детального анализа (может быть большим!)</p>
                        <a href="/debug/pprof/trace" target="_blank">Открыть trace</a>
                    </div>
                </div>
            </div>
            
            <div class="section">
                <h2>💡 Как использовать</h2>
                
                <div class="info">
                    <strong>📖 Базовое использование</strong>
                    Кликните на любую ссылку выше для просмотра профиля в браузере или используйте 
                    go tool pprof для детального анализа.
                </div>
                
                <h3>Анализ через командную строку:</h3>
                
                <div class="code-block"><code># Анализ CPU (30 секунд профилирования)
go tool pprof http://localhost:33000/debug/pprof/profile?seconds=30

# Анализ памяти
go tool pprof http://localhost:33000/debug/pprof/heap

# Анализ горутин
go tool pprof http://localhost:33000/debug/pprof/goroutine

# Сохранить профиль в файл
curl http://localhost:33000/debug/pprof/heap > heap.prof
go tool pprof heap.prof</code></div>
                
                <h3>Визуализация (требуется graphviz):</h3>
                
                <div class="code-block"><code># Установить graphviz
# macOS: brew install graphviz
# Ubuntu: sudo apt install graphviz

# Открыть в веб-интерфейсе
go tool pprof -http=:8080 http://localhost:33000/debug/pprof/heap</code></div>
                
                <h3>Полезные команды в интерактивном режиме pprof:</h3>
                
                <ul>
                    <li><code>top</code> - показать топ потребителей</li>
                    <li><code>top -cum</code> - топ по накопленному времени</li>
                    <li><code>list функция</code> - исходный код функции с аннотациями</li>
                    <li><code>web</code> - визуализация графа вызовов</li>
                    <li><code>png</code> - сохранить граф в PNG</li>
                    <li><code>help</code> - справка по командам</li>
                </ul>
            </div>
            
            <div class="section">
                <h2>📚 Документация</h2>
                <ul>
                    <li><a href="https://pkg.go.dev/net/http/pprof" target="_blank">
                        net/http/pprof - Официальная документация</a></li>
                    <li><a href="https://go.dev/blog/pprof" target="_blank">Profiling Go Programs</a></li>
                    <li><a href="https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/" target="_blank">
                        Profiling Go with pprof</a></li>
                </ul>
            </div>
        </div>
        
        <div class="footer">
            <p>OtusMS Microservice Debug Interface | Версия 1.0.0</p>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(html)); err != nil {
		s.logger.Error("Failed to write debug index response", "error", err)
	}
}
