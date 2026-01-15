# Superset Configuration for HelixAgent
# Apache Superset configuration file

import os
from datetime import timedelta
from cachelib.redis import RedisCache

# ----------------------------------------------------------
# Application Settings
# ----------------------------------------------------------

# Flask app name
APP_NAME = "HelixAgent Analytics"

# Flask secret key - MUST be set in production
SECRET_KEY = os.environ.get("SUPERSET_SECRET_KEY", "helixagent-superset-secret-key-change-in-production")

# ----------------------------------------------------------
# Database Settings
# ----------------------------------------------------------

# SQLAlchemy connection string
SQLALCHEMY_DATABASE_URI = (
    f"postgresql://{os.environ.get('DATABASE_USER', 'helixagent')}:"
    f"{os.environ.get('DATABASE_PASSWORD', 'helixagent123')}@"
    f"{os.environ.get('DATABASE_HOST', 'postgres')}:"
    f"{os.environ.get('DATABASE_PORT', '5432')}/"
    f"{os.environ.get('DATABASE_DB', 'superset')}"
)

# ----------------------------------------------------------
# Redis Configuration (for caching and Celery)
# ----------------------------------------------------------

REDIS_HOST = os.environ.get("REDIS_HOST", "redis")
REDIS_PORT = int(os.environ.get("REDIS_PORT", 6379))
REDIS_PASSWORD = os.environ.get("REDIS_PASSWORD", "helixagent123")
REDIS_DB = int(os.environ.get("REDIS_DB", 2))

# Redis cache configuration
CACHE_CONFIG = {
    "CACHE_TYPE": "RedisCache",
    "CACHE_DEFAULT_TIMEOUT": 300,
    "CACHE_KEY_PREFIX": "superset_",
    "CACHE_REDIS_HOST": REDIS_HOST,
    "CACHE_REDIS_PORT": REDIS_PORT,
    "CACHE_REDIS_PASSWORD": REDIS_PASSWORD,
    "CACHE_REDIS_DB": REDIS_DB,
}

DATA_CACHE_CONFIG = {
    "CACHE_TYPE": "RedisCache",
    "CACHE_DEFAULT_TIMEOUT": 86400,  # 24 hours
    "CACHE_KEY_PREFIX": "superset_data_",
    "CACHE_REDIS_HOST": REDIS_HOST,
    "CACHE_REDIS_PORT": REDIS_PORT,
    "CACHE_REDIS_PASSWORD": REDIS_PASSWORD,
    "CACHE_REDIS_DB": REDIS_DB,
}

FILTER_STATE_CACHE_CONFIG = {
    "CACHE_TYPE": "RedisCache",
    "CACHE_DEFAULT_TIMEOUT": 86400,
    "CACHE_KEY_PREFIX": "superset_filter_",
    "CACHE_REDIS_HOST": REDIS_HOST,
    "CACHE_REDIS_PORT": REDIS_PORT,
    "CACHE_REDIS_PASSWORD": REDIS_PASSWORD,
    "CACHE_REDIS_DB": REDIS_DB,
}

EXPLORE_FORM_DATA_CACHE_CONFIG = {
    "CACHE_TYPE": "RedisCache",
    "CACHE_DEFAULT_TIMEOUT": 86400,
    "CACHE_KEY_PREFIX": "superset_explore_",
    "CACHE_REDIS_HOST": REDIS_HOST,
    "CACHE_REDIS_PORT": REDIS_PORT,
    "CACHE_REDIS_PASSWORD": REDIS_PASSWORD,
    "CACHE_REDIS_DB": REDIS_DB,
}

# Results backend for async queries
RESULTS_BACKEND = RedisCache(
    host=REDIS_HOST,
    port=REDIS_PORT,
    password=REDIS_PASSWORD,
    db=REDIS_DB + 1,
    key_prefix="superset_results_",
)

# ----------------------------------------------------------
# Celery Configuration
# ----------------------------------------------------------

class CeleryConfig:
    broker_url = f"redis://:{REDIS_PASSWORD}@{REDIS_HOST}:{REDIS_PORT}/{REDIS_DB}"
    result_backend = f"redis://:{REDIS_PASSWORD}@{REDIS_HOST}:{REDIS_PORT}/{REDIS_DB}"
    imports = ("superset.sql_lab", "superset.tasks")
    task_annotations = {
        "sql_lab.get_sql_results": {"rate_limit": "100/s"},
    }
    worker_prefetch_multiplier = 1
    task_acks_late = True
    task_time_limit = 600
    task_soft_time_limit = 540

CELERY_CONFIG = CeleryConfig

# ----------------------------------------------------------
# Feature Flags
# ----------------------------------------------------------

FEATURE_FLAGS = {
    # Core features
    "ENABLE_TEMPLATE_PROCESSING": True,
    "ALERT_REPORTS": True,
    "DASHBOARD_NATIVE_FILTERS": True,
    "DASHBOARD_CROSS_FILTERS": True,
    "DASHBOARD_NATIVE_FILTERS_SET": True,
    "DASHBOARD_RBAC": True,
    "ENABLE_EXPLORE_DRAG_AND_DROP": True,
    "ENABLE_DND_WITH_CLICK_UX": True,

    # SQL Lab features
    "ESTIMATE_QUERY_COST": True,
    "ENABLE_TEMPLATE_REMOVE_FILTERS": True,
    "SQL_VALIDATORS_BY_ENGINE": True,

    # Embedding
    "EMBEDDABLE_CHARTS": True,
    "EMBEDDED_SUPERSET": True,

    # Advanced features
    "TAGGING_SYSTEM": True,
    "VERSIONED_EXPORT": True,
    "GLOBAL_ASYNC_QUERIES": True,
}

# ----------------------------------------------------------
# SQL Lab Settings
# ----------------------------------------------------------

# Enable async queries
SQLLAB_ASYNC_TIME_LIMIT_SEC = 600
SQLLAB_TIMEOUT = 300
SQL_MAX_ROW = 100000
DISPLAY_MAX_ROW = 10000

# Query validation
SQL_VALIDATORS_BY_ENGINE = {
    "postgresql": "PostgreSQLValidator",
}

# ----------------------------------------------------------
# Security Settings
# ----------------------------------------------------------

# CORS settings
ENABLE_CORS = True
CORS_OPTIONS = {
    "supports_credentials": True,
    "allow_headers": ["*"],
    "resources": ["*"],
    "origins": ["*"],
}

# Public role permissions (for embedded dashboards)
PUBLIC_ROLE_LIKE = "Gamma"

# Content Security Policy
TALISMAN_ENABLED = False  # Disable for development

# ----------------------------------------------------------
# Webserver Settings
# ----------------------------------------------------------

WEBSERVER_THREADS = 8
WEBSERVER_TIMEOUT = 300
SUPERSET_WEBSERVER_TIMEOUT = 300

# ----------------------------------------------------------
# Alert/Report Settings
# ----------------------------------------------------------

ALERT_REPORTS_NOTIFICATION_DRY_RUN = False
WEBDRIVER_BASEURL = "http://superset:8088/"
WEBDRIVER_BASEURL_USER_FRIENDLY = "http://localhost:8088/"

# Email configuration (configure for production)
SMTP_HOST = os.environ.get("SMTP_HOST", "localhost")
SMTP_PORT = int(os.environ.get("SMTP_PORT", 25))
SMTP_STARTTLS = os.environ.get("SMTP_STARTTLS", "false").lower() == "true"
SMTP_SSL = os.environ.get("SMTP_SSL", "false").lower() == "true"
SMTP_USER = os.environ.get("SMTP_USER", "")
SMTP_PASSWORD = os.environ.get("SMTP_PASSWORD", "")
SMTP_MAIL_FROM = os.environ.get("SMTP_MAIL_FROM", "superset@helixagent.ai")

# ----------------------------------------------------------
# Theme Configuration
# ----------------------------------------------------------

# Custom CSS
EXTRA_SEQUENTIAL_COLOR_SCHEMES = [
    {
        "id": "helixagent_blue",
        "description": "HelixAgent Blue Theme",
        "label": "HelixAgent Blue",
        "colors": [
            "#e8f4f8", "#c4e4f1", "#9fd3e9", "#6bbfdf",
            "#37abd5", "#1e8fc2", "#1574a9", "#0c5a8f", "#054175"
        ],
    },
]

EXTRA_CATEGORICAL_COLOR_SCHEMES = [
    {
        "id": "helixagent_categorical",
        "description": "HelixAgent Categorical",
        "label": "HelixAgent",
        "colors": [
            "#1e8fc2", "#37abd5", "#6bbfdf", "#ff6b6b",
            "#feca57", "#48dbfb", "#ff9ff3", "#54a0ff",
            "#5f27cd", "#00d2d3", "#c8d6e5", "#222f3e"
        ],
    },
]

# ----------------------------------------------------------
# Logging
# ----------------------------------------------------------

LOG_FORMAT = "%(asctime)s:%(levelname)s:%(name)s:%(message)s"
LOG_LEVEL = "INFO"
ENABLE_TIME_ROTATE = False

# ----------------------------------------------------------
# Database Connections (pre-configured)
# ----------------------------------------------------------

# These will be added programmatically by the init script
# - HelixAgent PostgreSQL (main database)
# - Iceberg Catalog (via Trino/Spark)
