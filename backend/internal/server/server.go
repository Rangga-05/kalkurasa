// Package server provides the shared Gin router used by both cmd/api (local)
// and Vercel's serverless entrypoint.
package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/catalog"
	"kalkurasa/backend/internal/config"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
	"kalkurasa/backend/internal/orders"
	"kalkurasa/backend/internal/settings"
	"kalkurasa/backend/internal/users"
)

// New creates and configures the Gin engine with all routes using a PostgreSQL
// user store when pool is non-nil, or UnavailableStore otherwise.
func New(cfg *config.Config, pool *pgxpool.Pool) *gin.Engine {
	var store auth.UserStore = auth.UnavailableStore{}
	if pool != nil {
		store = auth.PGStore{DB: pool}
	}
	return NewWithStore(cfg, pool, store)
}

// NewWithStore creates and configures the Gin engine with a custom auth.UserStore
// (used by unit tests to test authentication, RBAC, and validation without a DB).
func NewWithStore(cfg *config.Config, pool *pgxpool.Pool, store auth.UserStore) *gin.Engine {
	var dbConn db.Conn
	if pool != nil {
		dbConn = pool
	}
	return NewWithConn(cfg, pool, dbConn, store)
}

// NewWithConn creates and configures the Gin engine with custom db.Conn and auth.UserStore.
func NewWithConn(cfg *config.Config, pool *pgxpool.Pool, dbConn db.Conn, store auth.UserStore) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	origins := parseOrigins(cfg.AllowedOrigins)
	r.Use(httpx.CORS(origins))

	// --- Public health check (no DB dependency) ---
	r.GET("/api/healthz", func(c *gin.Context) {
		resp := gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		}

		// Optional DB ping with short timeout
		if pool != nil {
			dbCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()

			if err := pool.Ping(dbCtx); err != nil {
				resp["db"] = "error"
				resp["db_error"] = sanitizeDBError(err.Error())
			} else {
				resp["db"] = "ok"
			}
		} else {
			resp["db"] = "not_configured"
		}

		c.JSON(http.StatusOK, resp)
	})

	if store == nil {
		store = auth.UnavailableStore{}
	}
	secret := []byte(cfg.JWTSecret)
	authHandler := &auth.Handler{
		Store:        store,
		DB:           dbConn,
		Secret:       secret,
		CookieSecure: cfg.CookieSecure,
		ExposeToken:  cfg.AppEnv != "production",
	}
	guard := &auth.Guard{
		Store:          store,
		Secret:         secret,
		AllowedOrigins: origins,
	}
	settingsHandler := &settings.Handler{DB: dbConn}
	catalogHandler := &catalog.Handler{DB: dbConn}
	usersHandler := &users.Handler{DB: dbConn}
	ordersHandler := &orders.Handler{DB: dbConn}

	readMaster := auth.RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleAdmin)
	writeMaster := auth.RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleAdmin)
	readIngredients := auth.RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleAdmin, auth.RoleStaff)
	adminOnly := auth.RequireRole(auth.RoleAdmin)
	manageOrders := auth.RequireRole(auth.RoleOwner, auth.RoleManager)
	readOrders := auth.RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleStaff, auth.RoleAdmin)
	transitionOrders := auth.RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleStaff)
	customerOnly := auth.RequireRole(auth.RoleCustomer)

	v1 := r.Group("/api/v1")
	{
		// Public auth endpoints
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)
		v1.POST("/auth/logout", authHandler.Logout)

		// Public order & catalog endpoints (Tahap 3A, 3B & 4A+)
		v1.GET("/public/products", ordersHandler.PublicProducts)
		v1.POST("/public/availability", ordersHandler.PublicAvailability)
		v1.POST("/public/orders", guard.Authenticate(), customerOnly, ordersHandler.CreatePublicOrder)
		v1.GET("/public/orders/:code", ordersHandler.TrackPublicOrder)
		v1.POST("/public/orders/:code/counter-response", ordersHandler.CounterResponse)

		// Protected endpoints (require valid session cookie or Bearer token)
		protected := v1.Group("")
		protected.Use(guard.Authenticate())
		{
			protected.GET("/auth/me", authHandler.Me)
			protected.GET("/me/orders", customerOnly, ordersHandler.MyOrders)

			// Internal orders, decision queue & capacity (Tahap 3A & 3B)
			protected.GET("/orders", readOrders, ordersHandler.ListOrders)
			protected.GET("/orders/:id", readOrders, ordersHandler.GetOrder)
			protected.POST("/orders", manageOrders, ordersHandler.CreateManualOrder)
			protected.POST("/orders/:id/decision", manageOrders, ordersHandler.DecideOrder)
			protected.POST("/orders/:id/payments", manageOrders, ordersHandler.RecordPayment)
			protected.POST("/orders/:id/cancel", manageOrders, ordersHandler.CancelOrder)
			protected.POST("/orders/:id/transition", transitionOrders, ordersHandler.TransitionOrder)

			protected.GET("/decision-queue", manageOrders, ordersHandler.DecisionQueue)
			protected.GET("/capacity/calendar", manageOrders, ordersHandler.CapacityCalendar)
			protected.GET("/capacity/day/:date", manageOrders, ordersHandler.CapacityDay)

			// Operational tasks (Tahap 4B: owner, manager, staff; no prices/margins/cash)
			protected.GET("/tasks", transitionOrders, ordersHandler.ListTasks)
			protected.PATCH("/tasks/:id", transitionOrders, ordersHandler.UpdateTask)

			// Settings: owner full access; manager read-only without cash keys
			protected.GET("/settings",
				auth.RequireRole(auth.RoleOwner, auth.RoleManager),
				settingsHandler.Get,
			)
			protected.PUT("/settings",
				auth.RequireRole(auth.RoleOwner),
				settingsHandler.Put,
			)

			// Master Data: Products
			protected.GET("/products", readMaster, catalogHandler.ListProducts)
			protected.GET("/products/:id", readMaster, catalogHandler.GetProduct)
			protected.POST("/products", writeMaster, catalogHandler.CreateProduct)
			protected.PUT("/products/:id", writeMaster, catalogHandler.UpdateProduct)
			protected.DELETE("/products/:id", writeMaster, catalogHandler.DeleteProduct)
			protected.GET("/products/:id/recipes", readMaster, catalogHandler.ListRecipes)
			protected.PUT("/products/:id/recipes", writeMaster, catalogHandler.UpsertRecipe)
			protected.GET("/products/:id/loads", readMaster, catalogHandler.ListProductLoads)
			protected.PUT("/products/:id/loads", writeMaster, catalogHandler.UpsertProductLoad)

			// Master Data: Ingredients (staff allowed GET without buy_price)
			protected.GET("/ingredients", readIngredients, catalogHandler.ListIngredients)
			protected.GET("/ingredients/:id", readIngredients, catalogHandler.GetIngredient)
			protected.POST("/ingredients", writeMaster, catalogHandler.CreateIngredient)
			protected.PUT("/ingredients/:id", writeMaster, catalogHandler.UpdateIngredient)
			protected.DELETE("/ingredients/:id", writeMaster, catalogHandler.DeleteIngredient)

			// Master Data: Recipes (recipe_items)
			protected.GET("/recipes", readMaster, catalogHandler.ListRecipes)
			protected.POST("/recipes", writeMaster, catalogHandler.UpsertRecipe)
			protected.PUT("/recipes", writeMaster, catalogHandler.UpsertRecipe)
			protected.DELETE("/recipes/:product_id/:ingredient_id", writeMaster, catalogHandler.DeleteRecipeItem)

			// Master Data: Resources
			protected.GET("/resources", readMaster, catalogHandler.ListResources)
			protected.GET("/resources/:id", readMaster, catalogHandler.GetResource)
			protected.POST("/resources", writeMaster, catalogHandler.CreateResource)
			protected.PUT("/resources/:id", writeMaster, catalogHandler.UpdateResource)
			protected.DELETE("/resources/:id", writeMaster, catalogHandler.DeleteResource)

			// Master Data: Product Loads (product_loads)
			protected.GET("/product-loads", readMaster, catalogHandler.ListProductLoads)
			protected.POST("/product-loads", writeMaster, catalogHandler.UpsertProductLoad)
			protected.PUT("/product-loads", writeMaster, catalogHandler.UpsertProductLoad)
			protected.DELETE("/product-loads/:product_id/:resource_id", writeMaster, catalogHandler.DeleteProductLoad)

			// Users: admin only
			protected.GET("/users", adminOnly, usersHandler.List)
			protected.GET("/users/:id", adminOnly, usersHandler.Get)
			protected.POST("/users", adminOnly, usersHandler.Create)
			protected.PUT("/users/:id", adminOnly, usersHandler.Update)
			protected.PATCH("/users/:id", adminOnly, usersHandler.Update)
			protected.DELETE("/users/:id", adminOnly, usersHandler.Delete)

			// Shopping, purchases, stock adjustment, and product costing (Tahap 5)
			protected.GET("/shopping-list", readIngredients, ordersHandler.ShoppingList)
			protected.POST("/purchases", readIngredients, ordersHandler.RecordPurchase)
			protected.POST("/stock/adjust", readIngredients, ordersHandler.AdjustStock)
			protected.GET("/costing/products", readMaster, ordersHandler.ProductsCosting)

			// Cash projection & manual cash entries (Tahap 5; TC-09: staff denied access to cash)
			protected.GET("/cash/projection",
				auth.RequireRole(auth.RoleOwner, auth.RoleManager),
				ordersHandler.CashProjection,
			)
			protected.POST("/cash/entries",
				auth.RequireRole(auth.RoleOwner),
				ordersHandler.CreateCashEntry,
			)
		}
	}

	return r
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// sanitizeDBError removes any connection string, hostname, or password that
// might leak through a pgx error message. Returns a safe, short description.
func sanitizeDBError(raw string) string {
	// pgx errors may embed the DSN or hostname. Strip anything that looks
	// like a connection URI or contains sensitive keywords.
	lower := strings.ToLower(raw)
	for _, keyword := range []string{"password", "postgres://", "postgresql://", "@"} {
		if strings.Contains(lower, keyword) {
			return "koneksi database gagal"
		}
	}
	// Truncate to a reasonable length to avoid leaking internal details.
	if len(raw) > 120 {
		return raw[:120] + "…"
	}
	return raw
}
