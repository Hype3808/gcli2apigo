package dashboard

import (
	"gcli2apigo/internal/i18n"
	"html/template"
	"log"
	"net/http"
)

// loginTemplate is the embedded HTML template for the login page
var loginTemplate = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{index .T "login.title"}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0f0f0f;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }

        .login-container {
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 16px;
            padding: 48px;
            width: 100%;
            max-width: 420px;
        }

        .logo {
            text-align: center;
            margin-bottom: 40px;
        }

        .logo-icon {
            width: 56px;
            height: 56px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 28px;
            margin: 0 auto 16px;
        }

        .logo h1 {
            color: #ffffff;
            font-size: 24px;
            font-weight: 700;
            margin-bottom: 8px;
            letter-spacing: -0.5px;
        }

        .logo p {
            color: #888;
            font-size: 14px;
        }

        .error-message {
            background: rgba(220, 38, 38, 0.1);
            border: 1px solid #dc2626;
            color: #fca5a5;
            padding: 14px;
            border-radius: 8px;
            margin-bottom: 24px;
            font-size: 14px;
        }

        .form-group {
            margin-bottom: 24px;
        }

        label {
            display: block;
            color: #e0e0e0;
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 8px;
        }

        input[type="password"] {
            width: 100%;
            padding: 14px 16px;
            background: #0f0f0f;
            border: 1px solid #2a2a2a;
            border-radius: 8px;
            font-size: 15px;
            color: #e0e0e0;
            transition: all 0.2s;
        }

        input[type="password"]:focus {
            outline: none;
            border-color: #8b5cf6;
            background: #1a1a1a;
        }

        input[type="password"]::placeholder {
            color: #666;
        }

        .btn-login {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 15px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
            box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
        }

        .btn-login:hover {
            transform: translateY(-1px);
            box-shadow: 0 6px 20px rgba(139, 92, 246, 0.4);
        }

        .btn-login:active {
            transform: translateY(0);
        }



        @media (max-width: 768px) {
            .login-container {
                padding: 40px 24px;
                max-width: 100%;
            }

            .logo-icon {
                width: 48px;
                height: 48px;
                font-size: 24px;
            }

            .logo h1 {
                font-size: 22px;
            }

            .logo p {
                font-size: 13px;
            }
        }

        @media (max-width: 480px) {
            body {
                padding: 16px;
            }

            .login-container {
                padding: 32px 20px;
            }

            .logo-icon {
                width: 44px;
                height: 44px;
                font-size: 22px;
                margin-bottom: 12px;
            }

            .logo h1 {
                font-size: 20px;
            }

            .logo p {
                font-size: 12px;
            }

            input[type="password"] {
                padding: 12px 14px;
                font-size: 14px;
            }

            .btn-login {
                padding: 12px;
                font-size: 14px;
            }

            .error-message {
                padding: 12px;
                font-size: 13px;
            }
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">
            <div class="logo-icon">✨</div>
            <h1>{{index .T "login.heading"}}</h1>
            <p>{{index .T "login.subtitle"}}</p>
        </div>

        {{if .ErrorMessage}}
        <div class="error-message">
            {{.ErrorMessage}}
        </div>
        {{end}}

        <form method="POST" action="/dashboard/login">
            <div class="form-group">
                <label for="password">{{index .T "login.password"}}</label>
                <input 
                    type="password" 
                    id="password" 
                    name="password" 
                    placeholder="{{index .T "login.password.placeholder"}}"
                    required 
                    autofocus
                >
            </div>

            <button type="submit" class="btn-login">
                {{index .T "login.signin"}}
            </button>
        </form>
    </div>
</body>
</html>`

// dashboardTemplate is the embedded HTML template for the main dashboard page
var dashboardTemplate = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{index .T "dashboard.title"}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0f0f0f;
            color: #e0e0e0;
            min-height: 100vh;
        }

        .header {
            background: #1a1a1a;
            border-bottom: 1px solid #2a2a2a;
            padding: 0;
            position: sticky;
            top: 0;
            z-index: 100;
            backdrop-filter: blur(10px);
        }

        .header-content {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px 32px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .logo {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .logo-icon {
            width: 36px;
            height: 36px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 20px;
        }

        .logo-text h1 {
            font-size: 18px;
            font-weight: 600;
            color: #ffffff;
            letter-spacing: -0.5px;
        }

        .logo-text p {
            font-size: 12px;
            color: #888;
            margin-top: 2px;
        }

        .btn-logout {
            background: #2a2a2a;
            color: #e0e0e0;
            padding: 10px 20px;
            border: 1px solid #3a3a3a;
            border-radius: 8px;
            text-decoration: none;
            font-size: 14px;
            font-weight: 500;
            transition: all 0.2s;
            cursor: pointer;
        }

        .btn-logout:hover {
            background: #3a3a3a;
            border-color: #4a4a4a;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 32px;
        }

        .page-header {
            margin-bottom: 24px;
        }

        .page-title {
            font-size: 28px;
            font-weight: 700;
            color: #ffffff;
            margin-bottom: 8px;
            letter-spacing: -0.5px;
        }

        .page-subtitle {
            font-size: 14px;
            color: #888;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
            gap: 16px;
            margin-bottom: 32px;
        }

        .stat-card {
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 12px;
            padding: 20px;
            transition: all 0.2s;
        }

        .stat-card:hover {
            border-color: #3a3a3a;
            transform: translateY(-2px);
        }

        .stat-header {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 16px;
        }

        .stat-icon {
            width: 40px;
            height: 40px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 20px;
        }

        .stat-label {
            font-size: 13px;
            color: #888;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            font-weight: 600;
        }

        .stat-value {
            font-size: 32px;
            font-weight: 700;
            color: #ffffff;
            font-family: 'JetBrains Mono', 'Courier New', monospace;
            margin-bottom: 8px;
        }

        .stat-footer {
            font-size: 12px;
            color: #666;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .stat-footer .reset-time {
            color: #8b5cf6;
            font-weight: 600;
        }

        .actions {
            margin-bottom: 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 16px;
            padding: 20px;
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 12px;
        }

        .actions-left {
            display: flex;
            gap: 12px;
            align-items: center;
        }

        .dropdown {
            position: relative;
            display: inline-block;
        }

        .btn-add {
            display: inline-flex;
            align-items: center;
            gap: 10px;
            padding: 12px 24px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            transition: all 0.2s;
            border: none;
            cursor: pointer;
            box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
        }

        .btn-add:hover {
            transform: translateY(-1px);
            box-shadow: 0 6px 20px rgba(139, 92, 246, 0.4);
        }

        .btn-add:active {
            transform: translateY(0);
        }

        .dropdown-arrow {
            font-size: 10px;
            margin-left: 4px;
            transition: transform 0.2s;
        }

        .dropdown.active .dropdown-arrow {
            transform: rotate(180deg);
        }

        .dropdown-menu {
            position: absolute;
            top: calc(100% + 12px);
            left: 0;
            background: #1a1a1a;
            border: 1px solid #3a3a3a;
            border-radius: 12px;
            min-width: 300px;
            box-shadow: 0 12px 32px rgba(0, 0, 0, 0.5);
            opacity: 0;
            visibility: hidden;
            transform: translateY(-10px);
            transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
            z-index: 1000;
            overflow: hidden;
        }

        .dropdown.active .dropdown-menu {
            opacity: 1;
            visibility: visible;
            transform: translateY(0);
        }

        .dropdown-item {
            display: flex;
            align-items: center;
            gap: 16px;
            padding: 16px 20px;
            color: #e0e0e0;
            text-decoration: none;
            border: none;
            background: transparent;
            width: 100%;
            cursor: pointer;
            transition: all 0.2s;
            position: relative;
        }

        .dropdown-item::before {
            content: '';
            position: absolute;
            left: 0;
            top: 0;
            bottom: 0;
            width: 3px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            opacity: 0;
            transition: opacity 0.2s;
        }

        .dropdown-item:hover {
            background: rgba(139, 92, 246, 0.1);
        }

        .dropdown-item:hover::before {
            opacity: 1;
        }

        .dropdown-item:active {
            transform: scale(0.98);
        }

        .dropdown-icon {
            font-size: 28px;
            flex-shrink: 0;
            width: 40px;
            height: 40px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #2a2a2a;
            border-radius: 8px;
            transition: all 0.2s;
        }

        .dropdown-item:hover .dropdown-icon {
            background: rgba(139, 92, 246, 0.2);
            transform: scale(1.1);
        }

        .dropdown-item-content {
            flex: 1;
            text-align: left;
        }

        .dropdown-item-title {
            font-size: 15px;
            font-weight: 600;
            color: #ffffff;
            margin-bottom: 4px;
            letter-spacing: -0.2px;
        }

        .dropdown-item-desc {
            font-size: 12px;
            color: #888;
            line-height: 1.4;
        }

        .btn-select-all {
            display: inline-flex;
            align-items: center;
            gap: 10px;
            padding: 12px 24px;
            background: #2a2a2a;
            color: #e0e0e0;
            text-decoration: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            transition: all 0.2s;
            border: 1px solid #3a3a3a;
            cursor: pointer;
        }

        .btn-select-all:hover {
            background: #3a3a3a;
            border-color: #4a4a4a;
            transform: translateY(-1px);
        }

        .btn-select-all.all-selected {
            background: rgba(139, 92, 246, 0.2);
            border-color: #8b5cf6;
            color: #8b5cf6;
        }

        .btn-bulk-ban,
        .btn-bulk-unban {
            display: none;
            align-items: center;
            gap: 10px;
            padding: 12px 24px;
            background: #2a2a2a;
            color: #e0e0e0;
            text-decoration: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            transition: all 0.2s;
            border: 1px solid #3a3a3a;
            cursor: pointer;
        }

        .btn-bulk-ban.visible,
        .btn-bulk-unban.visible {
            display: inline-flex;
        }

        .btn-bulk-ban:hover,
        .btn-bulk-unban:hover {
            background: #3a3a3a;
            border-color: #4a4a4a;
            transform: translateY(-1px);
        }

        .btn-bulk-delete {
            display: none;
            align-items: center;
            gap: 10px;
            padding: 12px 24px;
            background: #dc2626;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            transition: all 0.2s;
            border: none;
            cursor: pointer;
        }

        .btn-bulk-delete.visible {
            display: inline-flex;
        }

        .btn-bulk-delete:hover {
            background: #b91c1c;
            transform: translateY(-1px);
        }

        .selection-info {
            color: #888;
            font-size: 14px;
            display: none;
            padding: 8px 16px;
            background: #2a2a2a;
            border-radius: 6px;
        }

        .selection-info.visible {
            display: block;
        }

        #selectedCount {
            color: #8b5cf6;
            font-weight: 600;
        }

        .empty-state {
            text-align: center;
            padding: 80px 40px;
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 16px;
        }

        .empty-state-icon {
            font-size: 64px;
            margin-bottom: 24px;
            opacity: 0.3;
        }

        .empty-state h2 {
            color: #ffffff;
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 12px;
        }

        .empty-state p {
            color: #888;
            font-size: 15px;
            line-height: 1.6;
        }

        .credentials-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 16px;
        }

        .credential-card {
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 12px;
            padding: 20px;
            transition: all 0.2s;
            position: relative;
            cursor: pointer;
        }

        .credential-card:hover {
            border-color: #3a3a3a;
            transform: translateY(-2px);
        }

        .credential-card.selected {
            border-color: #8b5cf6;
            background: rgba(139, 92, 246, 0.05);
        }

        .credential-card.banned {
            border-color: #dc2626;
            background: rgba(220, 38, 38, 0.05);
            opacity: 0.7;
        }

        .credential-card.banned .credential-icon {
            background: linear-gradient(135deg, #dc2626 0%, #b91c1c 100%);
        }

        .banned-badge {
            position: absolute;
            top: 12px;
            right: 12px;
            background: #dc2626;
            color: white;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 600;
            text-transform: uppercase;
            z-index: 10;
        }

        .error-status {
            display: flex;
            align-items: center;
            gap: 8px;
            margin-top: 12px;
            padding: 8px 12px;
            background: rgba(245, 158, 11, 0.1);
            border-left: 3px solid #f59e0b;
            border-radius: 4px;
        }

        .error-badge {
            background: #f59e0b;
            color: white;
            padding: 3px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 700;
            font-family: 'JetBrains Mono', 'Courier New', monospace;
            flex-shrink: 0;
        }

        .error-text {
            color: #f59e0b;
            font-size: 11px;
            font-weight: 600;
        }

        .credential-header {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .credential-checkbox {
            width: 18px;
            height: 18px;
            cursor: pointer;
            flex-shrink: 0;
            accent-color: #8b5cf6;
        }

        .credential-icon {
            width: 36px;
            height: 36px;
            background: linear-gradient(135deg, #8b5cf6 0%, #ec4899 100%);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 18px;
            flex-shrink: 0;
        }

        .credential-info {
            flex: 1;
            min-width: 0;
            padding-right: 100px;
        }

        .credential-project-id {
            color: #ffffff;
            font-size: 15px;
            font-weight: 600;
            word-break: break-all;
            font-family: 'JetBrains Mono', 'Courier New', monospace;
            letter-spacing: -0.3px;
        }

        .credential-expiry {
            margin-top: 8px;
            padding: 6px 10px;
            background: #2a2a2a;
            border-radius: 6px;
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 12px;
        }

        .credential-expiry.expired {
            background: rgba(220, 38, 38, 0.1);
            border-left: 3px solid #dc2626;
        }

        .credential-expiry.expiring-soon {
            background: rgba(245, 158, 11, 0.1);
            border-left: 3px solid #f59e0b;
        }

        .expiry-icon {
            font-size: 14px;
        }

        .expiry-label {
            color: #888;
            font-weight: 600;
        }

        .expiry-time {
            color: #8b5cf6;
            font-family: 'JetBrains Mono', 'Courier New', monospace;
            font-weight: 600;
        }

        .credential-expiry.expired .expiry-time {
            color: #dc2626;
        }

        .credential-expiry.expiring-soon .expiry-time {
            color: #f59e0b;
        }

        .credential-usage {
            margin-top: 12px;
        }

        .usage-item {
            margin-bottom: 12px;
        }

        .usage-item:last-child {
            margin-bottom: 0;
        }

        .usage-label {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 6px;
            font-size: 11px;
            color: #888;
        }

        .usage-count {
            font-family: 'JetBrains Mono', 'Courier New', monospace;
            font-size: 11px;
            font-weight: 600;
            color: #8b5cf6;
        }

        .usage-count.warning {
            color: #f59e0b;
        }

        .usage-count.danger {
            color: #dc2626;
        }

        .progress-bar-container {
            width: 100%;
            height: 4px;
            background: #2a2a2a;
            border-radius: 2px;
            overflow: hidden;
        }

        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, #8b5cf6 0%, #ec4899 100%);
            transition: width 0.3s ease-out;
            border-radius: 2px;
        }

        .progress-bar-fill.warning {
            background: linear-gradient(90deg, #f59e0b 0%, #f97316 100%);
        }

        .progress-bar-fill.danger {
            background: linear-gradient(90deg, #dc2626 0%, #b91c1c 100%);
        }

        .credential-actions {
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid #2a2a2a;
            display: flex;
            gap: 8px;
        }

        .btn-ban-single,
        .btn-unban-single {
            flex: 1;
            padding: 10px 16px;
            background: transparent;
            color: #888;
            border: 1px solid #3a3a3a;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-ban-single:hover {
            background: rgba(220, 38, 38, 0.1);
            border-color: #dc2626;
            color: #dc2626;
        }

        .btn-unban-single {
            color: #10b981;
            border-color: #10b981;
        }

        .btn-unban-single:hover {
            background: #10b981;
            color: white;
        }

        .btn-delete-single {
            flex: 1;
            padding: 10px 16px;
            background: transparent;
            color: #dc2626;
            border: 1px solid #dc2626;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-delete-single:hover {
            background: #dc2626;
            color: white;
        }

        .btn-ban-single:active,
        .btn-unban-single:active,
        .btn-delete-single:active {
            transform: scale(0.98);
        }

        .btn-delete {
            background: #fff;
            color: #f44336;
            border: 1px solid #f44336;
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.3s, color 0.3s;
            margin-top: 16px;
            width: 100%;
        }

        .btn-delete:hover {
            background: #f44336;
            color: white;
        }

        .btn-delete:active {
            transform: scale(0.98);
        }

        /* Upload Modal */
        .upload-modal {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.8);
            backdrop-filter: blur(4px);
            display: none;
            align-items: center;
            justify-content: center;
            z-index: 2000;
            padding: 20px;
        }

        .upload-modal.active {
            display: flex;
        }

        .upload-modal-content {
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            border-radius: 16px;
            width: 100%;
            max-width: 600px;
            max-height: 90vh;
            overflow-y: auto;
        }

        .upload-modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 24px;
            border-bottom: 1px solid #2a2a2a;
        }

        .upload-modal-header h3 {
            color: #ffffff;
            font-size: 20px;
            font-weight: 600;
        }

        .upload-modal-close {
            background: none;
            border: none;
            color: #888;
            font-size: 28px;
            cursor: pointer;
            padding: 0;
            width: 32px;
            height: 32px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: color 0.2s;
        }

        .upload-modal-close:hover {
            color: #e0e0e0;
        }

        .upload-modal-body {
            padding: 24px;
        }

        .upload-area {
            border: 2px dashed #3a3a3a;
            border-radius: 12px;
            padding: 48px 24px;
            text-align: center;
            transition: all 0.2s;
            cursor: pointer;
        }

        .upload-area:hover {
            border-color: #8b5cf6;
            background: rgba(139, 92, 246, 0.05);
        }

        .upload-area.drag-over {
            border-color: #8b5cf6;
            background: rgba(139, 92, 246, 0.1);
        }

        .upload-icon {
            font-size: 48px;
            margin-bottom: 16px;
        }

        .upload-title {
            color: #ffffff;
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 8px;
        }

        .upload-subtitle {
            color: #888;
            font-size: 14px;
            margin-bottom: 20px;
        }

        .btn-browse {
            padding: 10px 24px;
            background: #2a2a2a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }

        .btn-browse:hover {
            background: #3a3a3a;
            border-color: #4a4a4a;
        }

        .upload-info {
            margin-top: 16px;
            padding: 12px;
            background: #2a2a2a;
            border-radius: 8px;
            text-align: center;
        }

        .upload-info p {
            color: #888;
            font-size: 13px;
            margin: 0;
        }

        .upload-info strong {
            color: #8b5cf6;
        }

        .upload-files-list {
            margin-top: 20px;
        }

        .upload-file-item {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 12px;
            background: #2a2a2a;
            border-radius: 8px;
            margin-bottom: 8px;
        }

        .upload-file-icon {
            font-size: 24px;
            flex-shrink: 0;
        }

        .upload-file-info {
            flex: 1;
            min-width: 0;
        }

        .upload-file-name {
            color: #ffffff;
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 4px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .upload-file-size {
            color: #888;
            font-size: 12px;
        }

        .upload-file-status {
            flex-shrink: 0;
            font-size: 20px;
        }

        .upload-file-status.success {
            color: #10b981;
        }

        .upload-file-status.error {
            color: #dc2626;
        }

        .upload-file-status.uploading {
            color: #8b5cf6;
            animation: spin 1s linear infinite;
        }

        /* Responsive design */
        @media (max-width: 768px) {
            .header-content {
                padding: 16px 20px;
            }

            .logo-text h1 {
                font-size: 16px;
            }

            .logo-text p {
                display: none;
            }

            .container {
                padding: 20px 16px;
            }

            .page-title {
                font-size: 24px;
            }

            .page-subtitle {
                font-size: 13px;
            }

            .stats-grid {
                grid-template-columns: repeat(2, 1fr);
                gap: 12px;
            }

            .stat-card {
                padding: 16px;
            }

            .stat-icon {
                width: 36px;
                height: 36px;
                font-size: 18px;
            }

            .stat-label {
                font-size: 11px;
            }

            .stat-value {
                font-size: 24px;
            }

            .stat-footer {
                font-size: 11px;
            }

            .actions {
                flex-direction: column;
                align-items: stretch;
                padding: 16px;
            }

            .actions-left {
                flex-direction: column;
                width: 100%;
            }

            .btn-add,
            .btn-select-all,
            .btn-bulk-ban,
            .btn-bulk-unban,
            .btn-bulk-delete {
                width: 100%;
                justify-content: center;
            }

            .dropdown-menu {
                left: 0;
                right: 0;
                min-width: auto;
            }

            .credentials-grid {
                grid-template-columns: 1fr;
                gap: 12px;
            }

            .credential-card {
                padding: 16px;
            }

            .credential-info {
                padding-right: 90px;
            }

            .banned-badge {
                font-size: 10px;
                padding: 3px 6px;
                top: 10px;
                right: 10px;
            }

            .error-status {
                padding: 6px 10px;
                margin-top: 10px;
            }

            .error-badge {
                font-size: 11px;
                padding: 2px 6px;
            }

            .error-text {
                font-size: 10px;
            }

            .credential-project-id {
                font-size: 13px;
                word-break: break-word;
            }

            .usage-label {
                font-size: 10px;
            }

            .usage-count {
                font-size: 10px;
            }

            .credential-actions {
                flex-direction: column;
                gap: 6px;
            }

            .btn-ban-single,
            .btn-unban-single,
            .btn-delete-single {
                width: 100%;
                padding: 8px 12px;
                font-size: 12px;
            }

            .upload-modal-content {
                margin: 0 16px;
            }

            .upload-modal-header {
                padding: 20px;
            }

            .upload-modal-body {
                padding: 20px;
            }

            .upload-area {
                padding: 32px 16px;
            }

            .toast {
                bottom: 16px;
                right: 16px;
                left: 16px;
                min-width: auto;
            }

            .selection-info {
                font-size: 13px;
                padding: 6px 12px;
            }
        }

        @media (max-width: 480px) {
            .header-content {
                padding: 12px 16px;
            }

            .logo-icon {
                width: 32px;
                height: 32px;
                font-size: 18px;
            }

            .logo-text h1 {
                font-size: 14px;
            }

            .btn-logout {
                padding: 8px 16px;
                font-size: 13px;
            }

            .container {
                padding: 16px 12px;
            }

            .page-title {
                font-size: 20px;
            }

            .page-subtitle {
                font-size: 12px;
            }

            .stats-grid {
                grid-template-columns: 1fr;
                gap: 10px;
            }

            .stat-card {
                padding: 14px;
            }

            .stat-icon {
                width: 32px;
                height: 32px;
                font-size: 16px;
            }

            .stat-label {
                font-size: 10px;
            }

            .stat-value {
                font-size: 20px;
            }

            .stat-footer {
                font-size: 10px;
            }

            .actions {
                padding: 12px;
            }

            .credential-card {
                padding: 12px;
            }

            .credential-icon {
                width: 32px;
                height: 32px;
                font-size: 16px;
            }

            .credential-checkbox {
                width: 16px;
                height: 16px;
            }

            .credential-header {
                gap: 10px;
            }

            .credential-info {
                padding-right: 80px;
            }

            .credential-project-id {
                font-size: 12px;
            }

            .dropdown-item {
                padding: 12px 16px;
            }

            .dropdown-icon {
                width: 36px;
                height: 36px;
                font-size: 24px;
            }

            .dropdown-item-title {
                font-size: 14px;
            }

            .dropdown-item-desc {
                font-size: 11px;
            }
        }

        @media (min-width: 769px) and (max-width: 1200px) {
            .credentials-grid {
                grid-template-columns: repeat(2, 1fr);
            }
        }

        @media (min-width: 1201px) {
            .credentials-grid {
                grid-template-columns: repeat(3, 1fr);
            }
        }

        @media (min-width: 1600px) {
            .credentials-grid {
                grid-template-columns: repeat(4, 1fr);
            }
        }

        .success-message {
            background: #efe;
            border: 1px solid #cfc;
            color: #3c3;
            padding: 16px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 14px;
        }

        /* Loading overlay */
        .loading-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.8);
            backdrop-filter: blur(4px);
            display: none;
            align-items: center;
            justify-content: center;
            z-index: 1000;
        }

        .loading-overlay.active {
            display: flex;
        }

        .loading-spinner {
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            padding: 32px 48px;
            border-radius: 16px;
            text-align: center;
        }

        .spinner {
            width: 48px;
            height: 48px;
            border: 3px solid #2a2a2a;
            border-top: 3px solid #8b5cf6;
            border-radius: 50%;
            animation: spin 0.8s linear infinite;
            margin: 0 auto 20px;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        .loading-text {
            color: #e0e0e0;
            font-size: 15px;
            font-weight: 500;
        }

        /* Toast notification */
        .toast {
            position: fixed;
            bottom: 24px;
            right: 24px;
            background: #1a1a1a;
            border: 1px solid #2a2a2a;
            padding: 16px 20px;
            border-radius: 12px;
            display: none;
            align-items: center;
            gap: 12px;
            z-index: 1001;
            min-width: 320px;
            animation: slideIn 0.3s ease-out;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
        }

        @keyframes slideIn {
            from {
                transform: translateX(400px);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }

        .toast.active {
            display: flex;
        }

        .toast.error {
            border-left: 3px solid #dc2626;
        }

        .toast.success {
            border-left: 3px solid #10b981;
        }

        .toast.warning {
            border-left: 3px solid #f59e0b;
        }

        .toast-icon {
            font-size: 20px;
        }

        .toast-message {
            flex: 1;
            color: #e0e0e0;
            font-size: 14px;
            line-height: 1.5;
        }

        .toast-close {
            background: none;
            border: none;
            color: #666;
            font-size: 18px;
            cursor: pointer;
            padding: 0;
            width: 24px;
            height: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: color 0.2s;
        }

        .toast-close:hover {
            color: #333;
        }

        ` + languageSwitcherCSS + `
    </style>
</head>
<body>
    <header class="header">
        <div class="header-content">
            <div class="logo">
                <div class="logo-icon">✨</div>
                <div class="logo-text">
                    <h1>{{index .T "dashboard.heading"}}</h1>
                    <p>{{index .T "dashboard.subtitle"}}</p>
                </div>
            </div>
            <div style="display: flex; align-items: center; gap: 12px;">
                ` + languageSwitcherHTML + `
                <a href="/dashboard/logout" class="btn-logout">{{index .T "dashboard.logout"}}</a>
            </div>
        </div>
    </header>

    <div class="container">
        <div class="page-header">
            <h2 class="page-title">{{index .T "dashboard.page.title"}}</h2>
            <p class="page-subtitle">{{index .T "dashboard.page.subtitle"}}</p>
        </div>

        <!-- Stats Cards -->
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-header">
                    <div class="stat-icon">🚀</div>
                    <div class="stat-label">{{index .T "stats.pro.label"}}</div>
                </div>
                <div class="stat-value" id="statProRequests">-</div>
                <div class="stat-footer">
                    <span>{{index .T "stats.pro.footer"}} <span class="reset-time" id="resetTime1">-</span></span>
                </div>
            </div>

            <div class="stat-card">
                <div class="stat-header">
                    <div class="stat-icon">📊</div>
                    <div class="stat-label">{{index .T "stats.total.label"}}</div>
                </div>
                <div class="stat-value" id="statTotalRequests">-</div>
                <div class="stat-footer">
                    <span>{{index .T "stats.total.footer"}} <span class="reset-time" id="resetTime2">-</span></span>
                </div>
            </div>

            <div class="stat-card">
                <div class="stat-header">
                    <div class="stat-icon">⚡</div>
                    <div class="stat-label">{{index .T "stats.rpm.label"}}</div>
                </div>
                <div class="stat-value" id="statRPM">-</div>
                <div class="stat-footer">
                    <span>{{index .T "stats.rpm.footer"}}</span>
                </div>
            </div>

            <div class="stat-card">
                <div class="stat-header">
                    <div class="stat-icon">🔑</div>
                    <div class="stat-label">{{index .T "stats.active.label"}}</div>
                </div>
                <div class="stat-value" id="statActiveCredentials">-</div>
                <div class="stat-footer">
                    <span>{{index .T "stats.active.footer"}}</span>
                </div>
            </div>
        </div>

        <div class="actions">
            <div class="actions-left">
                <div class="dropdown">
                    <button class="btn-add dropdown-toggle" id="addCredentialBtn">
                        <span>+</span>
                        <span>{{index .T "actions.add"}}</span>
                        <span class="dropdown-arrow">▼</span>
                    </button>
                    <div class="dropdown-menu" id="addCredentialMenu">
                        <a href="/dashboard/oauth/start" class="dropdown-item" id="oauthFlowBtn">
                            <span class="dropdown-icon">🔐</span>
                            <div class="dropdown-item-content">
                                <div class="dropdown-item-title">{{index .T "add.oauth.title"}}</div>
                                <div class="dropdown-item-desc">{{index .T "add.oauth.desc"}}</div>
                            </div>
                        </a>
                        <button class="dropdown-item" id="uploadCredentialBtn">
                            <span class="dropdown-icon">📁</span>
                            <div class="dropdown-item-content">
                                <div class="dropdown-item-title">{{index .T "add.upload.title"}}</div>
                                <div class="dropdown-item-desc">{{index .T "add.upload.desc"}}</div>
                            </div>
                        </button>
                    </div>
                </div>
                <button class="btn-select-all" id="selectAllBtn" data-select-text="{{index .T "actions.select.all"}}" data-deselect-text="{{index .T "actions.deselect.all"}}">
                    <span>☑</span>
                    <span id="selectAllText">{{index .T "actions.select.all"}}</span>
                </button>
                <button class="btn-bulk-ban" id="bulkBanBtn">
                    <span>🚫</span>
                    <span>{{index .T "actions.ban.selected"}}</span>
                </button>
                <button class="btn-bulk-unban" id="bulkUnbanBtn">
                    <span>✓</span>
                    <span>{{index .T "actions.unban.selected"}}</span>
                </button>
                <button class="btn-bulk-delete" id="bulkDeleteBtn">
                    <span>×</span>
                    <span>{{index .T "actions.delete.selected"}}</span>
                </button>
            </div>
            <div class="selection-info" id="selectionInfo">
                <span id="selectedCount">0</span> {{index .T "actions.selected.count"}}
            </div>
        </div>

        <!-- Upload Modal -->
        <div class="upload-modal" id="uploadModal">
            <div class="upload-modal-content">
                <div class="upload-modal-header">
                    <h3>{{index .T "upload.title"}}</h3>
                    <button class="upload-modal-close" id="uploadModalClose">×</button>
                </div>
                <div class="upload-modal-body">
                    <div class="upload-area" id="uploadArea">
                        <div class="upload-icon">📤</div>
                        <div class="upload-text">
                            <p class="upload-title">{{index .T "upload.drag"}}</p>
                            <p class="upload-subtitle">{{index .T "upload.or"}}</p>
                        </div>
                        <input type="file" id="fileInput" accept=".json,.zip" multiple hidden>
                        <button class="btn-browse" id="browseBtn">{{index .T "upload.browse"}}</button>
                    </div>
                    <div class="upload-info">
                        <p>{{index .T "upload.info"}} <strong>{{index .T "upload.json"}}</strong> {{index .T "upload.json.desc"}} {{index .T "upload.or"}} <strong>{{index .T "upload.zip"}}</strong> {{index .T "upload.zip.desc"}}</p>
                    </div>
                    <div class="upload-files-list" id="uploadFilesList"></div>
                </div>
            </div>
        </div>

        {{if .Credentials}}
        <div class="credentials-grid">
            {{range .Credentials}}
            <div class="credential-card {{if .IsBanned}}banned{{end}}" data-project-id="{{.ProjectID}}" data-banned="{{.IsBanned}}">
                {{if .IsBanned}}<div class="banned-badge">{{index $.T "credential.banned"}}</div>{{end}}
                <div class="credential-header">
                    <input type="checkbox" class="credential-checkbox" data-project-id="{{.ProjectID}}">
                    <div class="credential-icon">🔑</div>
                    <div class="credential-info">
                        <div class="credential-project-id">{{.ProjectID}}</div>
                    </div>
                </div>
                
                {{if not .Expiry.IsZero}}
                <div class="credential-expiry" data-expiry="{{.Expiry.Format "2006-01-02T15:04:05Z07:00"}}">
                    <span class="expiry-icon">⏰</span>
                    <span class="expiry-label">Expires:</span>
                    <span class="expiry-time">{{.Expiry.Format "2006-01-02 15:04:05"}}</span>
                </div>
                {{end}}
                
                {{if gt .LastErrorCode 0}}
                <div class="error-status">
                    <span class="error-badge">{{.LastErrorCode}}</span>
                    <span class="error-text">{{index $.T "credential.error"}}</span>
                </div>
                {{end}}
                
                <div class="credential-usage">
                    <div class="usage-item">
                        <div class="usage-label">
                            <span>{{index $.T "credential.pro.models"}}</span>
                            <span class="usage-count {{if ge .ProModelCount 90}}danger{{else if ge .ProModelCount 70}}warning{{end}}">
                                {{.ProModelCount}} / {{.ProModelLimit}}
                            </span>
                        </div>
                        <div class="progress-bar-container">
                            <div class="progress-bar-fill {{if ge .ProModelCount 90}}danger{{else if ge .ProModelCount 70}}warning{{end}}" 
                                 data-count="{{.ProModelCount}}" data-limit="{{.ProModelLimit}}">
                            </div>
                        </div>
                    </div>
                    
                    <div class="usage-item">
                        <div class="usage-label">
                            <span>{{index $.T "credential.all.models"}}</span>
                            <span class="usage-count {{if ge .OverallCount 900}}danger{{else if ge .OverallCount 700}}warning{{end}}">
                                {{.OverallCount}} / {{.OverallLimit}}
                            </span>
                        </div>
                        <div class="progress-bar-container">
                            <div class="progress-bar-fill {{if ge .OverallCount 900}}danger{{else if ge .OverallCount 700}}warning{{end}}" 
                                 data-count="{{.OverallCount}}" data-limit="{{.OverallLimit}}">
                            </div>
                        </div>
                    </div>
                </div>
                
                <div class="credential-actions">
                    {{if .IsBanned}}
                    <button class="btn-unban-single" data-project-id="{{.ProjectID}}">
                        {{index $.T "credential.unban"}}
                    </button>
                    {{else}}
                    <button class="btn-ban-single" data-project-id="{{.ProjectID}}">
                        {{index $.T "credential.ban"}}
                    </button>
                    {{end}}
                    <button class="btn-delete-single" data-project-id="{{.ProjectID}}">
                        {{index $.T "credential.delete"}}
                    </button>
                </div>
            </div>
            {{end}}
        </div>
        {{else}}
        <div class="empty-state">
            <div class="empty-state-icon">📭</div>
            <h2>{{index .T "empty.title"}}</h2>
            <p>{{index .T "empty.message"}}</p>
        </div>
        {{end}}
    </div>

    <!-- Loading Overlay -->
    <div class="loading-overlay" id="loadingOverlay">
        <div class="loading-spinner">
            <div class="spinner"></div>
            <div class="loading-text" id="loadingText">Processing...</div>
        </div>
    </div>

    <!-- Toast Notification -->
    <div class="toast" id="toast">
        <div class="toast-icon" id="toastIcon"></div>
        <div class="toast-message" id="toastMessage"></div>
        <button class="toast-close" id="toastClose">×</button>
    </div>

    <script>
        ` + languageSwitcherJS + `

        // Toast notification system
        const toast = {
            element: document.getElementById('toast'),
            icon: document.getElementById('toastIcon'),
            message: document.getElementById('toastMessage'),
            closeBtn: document.getElementById('toastClose'),
            timeout: null,

            show: function(message, type = 'success') {
                this.element.className = 'toast active ' + type;
                this.icon.textContent = type === 'success' ? '✓' : '✗';
                this.message.textContent = message;

                // Clear existing timeout
                if (this.timeout) {
                    clearTimeout(this.timeout);
                }

                // Auto-hide after 5 seconds
                this.timeout = setTimeout(() => {
                    this.hide();
                }, 5000);
            },

            hide: function() {
                this.element.classList.remove('active');
                if (this.timeout) {
                    clearTimeout(this.timeout);
                    this.timeout = null;
                }
            }
        };

        // Close toast on click
        toast.closeBtn.addEventListener('click', () => {
            toast.hide();
        });

        // Loading overlay
        const loading = {
            overlay: document.getElementById('loadingOverlay'),
            text: document.getElementById('loadingText'),

            show: function(message = 'Processing...') {
                this.text.textContent = message;
                this.overlay.classList.add('active');
            },

            hide: function() {
                this.overlay.classList.remove('active');
            }
        };

        // Selection management
        const selectedProjects = new Set();
        const bulkDeleteBtn = document.getElementById('bulkDeleteBtn');
        const bulkBanBtn = document.getElementById('bulkBanBtn');
        const bulkUnbanBtn = document.getElementById('bulkUnbanBtn');
        const selectAllBtn = document.getElementById('selectAllBtn');
        const selectionInfo = document.getElementById('selectionInfo');
        const selectedCount = document.getElementById('selectedCount');

        function updateSelectionUI() {
            const count = selectedProjects.size;
            selectedCount.textContent = count;
            
            if (count > 0) {
                bulkDeleteBtn.classList.add('visible');
                bulkBanBtn.classList.add('visible');
                bulkUnbanBtn.classList.add('visible');
                selectionInfo.classList.add('visible');
            } else {
                bulkDeleteBtn.classList.remove('visible');
                bulkBanBtn.classList.remove('visible');
                bulkUnbanBtn.classList.remove('visible');
                selectionInfo.classList.remove('visible');
            }

            // Update select all button state
            const allCheckboxes = document.querySelectorAll('.credential-checkbox');
            const allSelected = allCheckboxes.length > 0 && count === allCheckboxes.length;
            
            const selectText = selectAllBtn.getAttribute('data-select-text');
            const deselectText = selectAllBtn.getAttribute('data-deselect-text');
            
            if (allSelected) {
                selectAllBtn.classList.add('all-selected');
                document.getElementById('selectAllText').textContent = deselectText;
            } else {
                selectAllBtn.classList.remove('all-selected');
                document.getElementById('selectAllText').textContent = selectText;
            }
        }

        function selectAll() {
            const checkboxes = document.querySelectorAll('.credential-checkbox');
            checkboxes.forEach(checkbox => {
                const projectId = checkbox.getAttribute('data-project-id');
                const card = checkbox.closest('.credential-card');
                
                if (!checkbox.checked) {
                    checkbox.checked = true;
                    selectedProjects.add(projectId);
                    card.classList.add('selected');
                }
            });
            updateSelectionUI();
        }

        function deselectAll() {
            const checkboxes = document.querySelectorAll('.credential-checkbox');
            checkboxes.forEach(checkbox => {
                const projectId = checkbox.getAttribute('data-project-id');
                const card = checkbox.closest('.credential-card');
                
                if (checkbox.checked) {
                    checkbox.checked = false;
                    selectedProjects.delete(projectId);
                    card.classList.remove('selected');
                }
            });
            updateSelectionUI();
        }

        function toggleCardSelection(projectId, checkbox, card) {
            if (checkbox.checked) {
                selectedProjects.add(projectId);
                card.classList.add('selected');
            } else {
                selectedProjects.delete(projectId);
                card.classList.remove('selected');
            }
            updateSelectionUI();
        }

        // Bulk delete function
        function bulkDeleteCredentials(projectIds) {
            loading.show('Deleting ' + projectIds.length + ' credential(s)...');
            
            let completed = 0;
            let failed = 0;
            
            const deletePromises = projectIds.map(projectId => {
                return fetch('/dashboard/api/credentials/' + encodeURIComponent(projectId), {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' }
                })
                .then(response => {
                    if (!response.ok) {
                        return response.json().then(data => {
                            throw new Error(data.error || 'Failed to delete');
                        });
                    }
                    return response.json();
                })
                .then(data => {
                    if (data.success) {
                        completed++;
                        // Remove card from DOM
                        const card = document.querySelector('.credential-card[data-project-id="' + projectId + '"]');
                        if (card) {
                            card.style.transform = 'scale(0)';
                            card.style.opacity = '0';
                            card.style.transition = 'transform 0.3s ease-out, opacity 0.3s ease-out';
                            setTimeout(() => card.remove(), 300);
                        }
                    }
                })
                .catch(error => {
                    failed++;
                    console.error('Failed to delete ' + projectId + ':', error);
                });
            });
            
            Promise.all(deletePromises).then(() => {
                loading.hide();
                
                if (failed === 0) {
                    toast.show('Successfully deleted ' + completed + ' credential(s)', 'success');
                } else if (completed > 0) {
                    toast.show('Deleted ' + completed + ' credential(s), ' + failed + ' failed', 'warning');
                } else {
                    toast.show('Failed to delete credentials', 'error');
                }
                
                // Clear selection
                selectedProjects.clear();
                updateSelectionUI();
                
                // Check if grid is empty
                setTimeout(() => {
                    const grid = document.querySelector('.credentials-grid');
                    if (grid && grid.children.length === 0) {
                        window.location.reload();
                    }
                }, 500);
            });
        }

        // Ban/Unban functions
        function banCredentials(projectIds) {
            loading.show('Banning ' + projectIds.length + ' credential(s)...');
            
            fetch('/dashboard/api/credentials/ban', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ project_ids: projectIds })
            })
            .then(response => response.json())
            .then(data => {
                loading.hide();
                if (data.success) {
                    toast.show(data.message, 'success');
                    setTimeout(() => window.location.reload(), 1000);
                } else {
                    toast.show(data.error || 'Failed to ban credentials', 'error');
                }
            })
            .catch(error => {
                loading.hide();
                toast.show('Failed to ban credentials: ' + error.message, 'error');
            });
        }

        function unbanCredentials(projectIds) {
            loading.show('Unbanning ' + projectIds.length + ' credential(s)...');
            
            fetch('/dashboard/api/credentials/unban', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ project_ids: projectIds })
            })
            .then(response => response.json())
            .then(data => {
                loading.hide();
                if (data.success) {
                    toast.show(data.message, 'success');
                    setTimeout(() => window.location.reload(), 1000);
                } else {
                    toast.show(data.error || 'Failed to unban credentials', 'error');
                }
            })
            .catch(error => {
                loading.hide();
                toast.show('Failed to unban credentials: ' + error.message, 'error');
            });
        }

        // Single delete function
        function deleteSingleCredential(projectId) {
            loading.show('Deleting credential...');
            
            fetch('/dashboard/api/credentials/' + encodeURIComponent(projectId), {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' }
            })
            .then(response => {
                if (!response.ok) {
                    return response.json().then(data => {
                        throw new Error(data.error || 'Failed to delete credential');
                    });
                }
                return response.json();
            })
            .then(data => {
                loading.hide();
                
                if (data.success) {
                    toast.show('Credential deleted successfully', 'success');
                    
                    // Remove card from DOM
                    const card = document.querySelector('.credential-card[data-project-id="' + projectId + '"]');
                    if (card) {
                        card.style.transform = 'scale(0)';
                        card.style.opacity = '0';
                        card.style.transition = 'transform 0.3s ease-out, opacity 0.3s ease-out';
                        
                        setTimeout(() => {
                            card.remove();
                            
                            // Remove from selection if selected
                            selectedProjects.delete(projectId);
                            updateSelectionUI();
                            
                            // Check if grid is empty
                            const grid = document.querySelector('.credentials-grid');
                            if (grid && grid.children.length === 0) {
                                setTimeout(() => window.location.reload(), 500);
                            }
                        }, 300);
                    }
                } else {
                    throw new Error(data.error || 'Unknown error');
                }
            })
            .catch(error => {
                loading.hide();
                toast.show('Failed to delete credential: ' + error.message, 'error');
                console.error('Delete error:', error);
            });
        }

        // Initialize progress bars with animation
        function initializeProgressBars() {
            const progressBars = document.querySelectorAll('.progress-bar-fill');
            progressBars.forEach(bar => {
                const count = parseInt(bar.getAttribute('data-count')) || 0;
                const limit = parseInt(bar.getAttribute('data-limit')) || 1;
                const percentage = Math.min(100, (count / limit) * 100);
                
                // Animate from 0 to target percentage
                setTimeout(() => {
                    bar.style.width = percentage + '%';
                }, 100);
            });
        }

        // Update expiry times and status
        function updateExpiryTimes() {
            const expiryElements = document.querySelectorAll('.credential-expiry');
            const now = new Date();
            
            expiryElements.forEach(element => {
                const expiryStr = element.getAttribute('data-expiry');
                if (!expiryStr) return;
                
                const expiryDate = new Date(expiryStr);
                const timeUntilExpiry = expiryDate - now;
                const hoursUntilExpiry = timeUntilExpiry / (1000 * 60 * 60);
                
                // Update status classes
                element.classList.remove('expired', 'expiring-soon');
                
                if (timeUntilExpiry < 0) {
                    element.classList.add('expired');
                    const timeLabel = element.querySelector('.expiry-label');
                    if (timeLabel) timeLabel.textContent = 'Expired:';
                } else if (hoursUntilExpiry < 24) {
                    element.classList.add('expiring-soon');
                    const timeLabel = element.querySelector('.expiry-label');
                    if (timeLabel) timeLabel.textContent = 'Expires in:';
                    
                    // Show relative time for expiring soon
                    const timeSpan = element.querySelector('.expiry-time');
                    if (timeSpan && hoursUntilExpiry < 24) {
                        const hours = Math.floor(hoursUntilExpiry);
                        const minutes = Math.floor((hoursUntilExpiry - hours) * 60);
                        if (hours > 0) {
                            timeSpan.textContent = hours + 'h ' + minutes + 'm';
                        } else {
                            timeSpan.textContent = minutes + 'm';
                        }
                    }
                }
            });
        }

        // Dropdown functionality
        const dropdown = document.querySelector('.dropdown');
        const dropdownToggle = document.getElementById('addCredentialBtn');
        const dropdownMenu = document.getElementById('addCredentialMenu');

        if (dropdownToggle) {
            dropdownToggle.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                dropdown.classList.toggle('active');
            });
        }

        // Close dropdown when clicking outside
        document.addEventListener('click', (e) => {
            if (dropdown && !dropdown.contains(e.target)) {
                dropdown.classList.remove('active');
            }
        });

        // Upload modal functionality
        const uploadModal = document.getElementById('uploadModal');
        const uploadModalClose = document.getElementById('uploadModalClose');
        const uploadCredentialBtn = document.getElementById('uploadCredentialBtn');
        const uploadArea = document.getElementById('uploadArea');
        const fileInput = document.getElementById('fileInput');
        const browseBtn = document.getElementById('browseBtn');
        const uploadFilesList = document.getElementById('uploadFilesList');

        // OAuth flow button
        const oauthFlowBtn = document.getElementById('oauthFlowBtn');
        if (oauthFlowBtn) {
            oauthFlowBtn.addEventListener('click', () => {
                loading.show('Redirecting to Google OAuth...');
            });
        }

        // Open upload modal
        if (uploadCredentialBtn) {
            uploadCredentialBtn.addEventListener('click', () => {
                dropdown.classList.remove('active');
                uploadModal.classList.add('active');
            });
        }

        // Close upload modal
        if (uploadModalClose) {
            uploadModalClose.addEventListener('click', () => {
                uploadModal.classList.remove('active');
                uploadFilesList.innerHTML = '';
            });
        }

        // Close modal on background click
        uploadModal.addEventListener('click', (e) => {
            if (e.target === uploadModal) {
                uploadModal.classList.remove('active');
                uploadFilesList.innerHTML = '';
            }
        });

        // Browse button
        if (browseBtn) {
            browseBtn.addEventListener('click', (e) => {
                e.stopPropagation(); // Prevent event from bubbling to uploadArea
                fileInput.click();
            });
        }

        // Click upload area to browse
        if (uploadArea) {
            uploadArea.addEventListener('click', (e) => {
                // Only trigger if clicking the area itself, not the button
                if (e.target === uploadArea || e.target.closest('.upload-text') || e.target.closest('.upload-icon')) {
                    fileInput.click();
                }
            });
        }

        // Drag and drop
        uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadArea.classList.add('drag-over');
        });

        uploadArea.addEventListener('dragleave', () => {
            uploadArea.classList.remove('drag-over');
        });

        uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadArea.classList.remove('drag-over');
            handleFiles(e.dataTransfer.files);
        });

        // File input change
        fileInput.addEventListener('change', (e) => {
            handleFiles(e.target.files);
        });

        // Handle file uploads
        function handleFiles(files) {
            uploadFilesList.innerHTML = '';
            
            Array.from(files).forEach(file => {
                const fileItem = createFileItem(file);
                uploadFilesList.appendChild(fileItem);
                uploadFile(file, fileItem);
            });
        }

        function createFileItem(file) {
            const item = document.createElement('div');
            item.className = 'upload-file-item';
            
            const icon = file.name.endsWith('.zip') ? '📦' : '📄';
            const size = formatFileSize(file.size);
            
            item.innerHTML = '<div class="upload-file-icon">' + icon + '</div>' +
                '<div class="upload-file-info">' +
                '<div class="upload-file-name">' + file.name + '</div>' +
                '<div class="upload-file-size">' + size + '</div>' +
                '</div>' +
                '<div class="upload-file-status uploading">⏳</div>';
            
            return item;
        }

        function formatFileSize(bytes) {
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
            return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        }

        function uploadFile(file, fileItem) {
            const formData = new FormData();
            formData.append('file', file);
            
            fetch('/dashboard/api/credentials/upload', {
                method: 'POST',
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                const statusEl = fileItem.querySelector('.upload-file-status');
                if (data.success) {
                    statusEl.className = 'upload-file-status success';
                    statusEl.textContent = '✓';
                    toast.show(data.message || 'File uploaded successfully', 'success');
                    
                    // Reload page after a short delay
                    setTimeout(() => {
                        window.location.reload();
                    }, 1500);
                } else {
                    statusEl.className = 'upload-file-status error';
                    statusEl.textContent = '✗';
                    toast.show(data.error || 'Upload failed', 'error');
                }
            })
            .catch(error => {
                const statusEl = fileItem.querySelector('.upload-file-status');
                statusEl.className = 'upload-file-status error';
                statusEl.textContent = '✗';
                toast.show('Upload failed: ' + error.message, 'error');
            });
        }

        // Fetch and update dashboard stats
        function updateDashboardStats() {
            fetch('/dashboard/api/stats')
                .then(response => response.json())
                .then(data => {
                    // Update stat values
                    document.getElementById('statProRequests').textContent = data.total_pro_requests.toLocaleString();
                    document.getElementById('statTotalRequests').textContent = data.total_overall_requests.toLocaleString();
                    document.getElementById('statRPM').textContent = data.rpm.toFixed(2);
                    document.getElementById('statActiveCredentials').textContent = data.active_credentials.toLocaleString();
                    
                    // Format reset time
                    const resetTime = new Date(data.next_reset_time);
                    const timeStr = resetTime.toLocaleTimeString('en-US', { 
                        hour: '2-digit', 
                        minute: '2-digit',
                        hour12: false 
                    });
                    document.getElementById('resetTime1').textContent = timeStr;
                    document.getElementById('resetTime2').textContent = timeStr;
                })
                .catch(error => {
                    console.error('Failed to fetch dashboard stats:', error);
                });
        }

        // Attach event listeners
        document.addEventListener('DOMContentLoaded', () => {
            // Load dashboard stats
            updateDashboardStats();
            
            // Refresh stats every 30 seconds
            setInterval(updateDashboardStats, 30000);
            
            // Initialize progress bars
            initializeProgressBars();
            
            // Initialize and update expiry times
            updateExpiryTimes();
            setInterval(updateExpiryTimes, 60000); // Update every minute
            // Checkbox listeners
            const checkboxes = document.querySelectorAll('.credential-checkbox');
            checkboxes.forEach(checkbox => {
                const projectId = checkbox.getAttribute('data-project-id');
                const card = checkbox.closest('.credential-card');
                
                checkbox.addEventListener('change', (e) => {
                    e.stopPropagation();
                    toggleCardSelection(projectId, checkbox, card);
                });
                
                // Allow clicking card to toggle checkbox (but not delete button)
                card.addEventListener('click', (e) => {
                    // Don't toggle if clicking checkbox or delete button
                    if (e.target !== checkbox && !e.target.classList.contains('btn-delete-single')) {
                        checkbox.checked = !checkbox.checked;
                        toggleCardSelection(projectId, checkbox, card);
                    }
                });
            });

            // Single ban button listeners
            const banButtons = document.querySelectorAll('.btn-ban-single');
            banButtons.forEach(button => {
                button.addEventListener('click', (e) => {
                    e.stopPropagation();
                    const projectId = button.getAttribute('data-project-id');
                    
                    if (confirm('Are you sure you want to ban credential for project: ' + projectId + '?')) {
                        banCredentials([projectId]);
                    }
                });
            });

            // Single unban button listeners
            const unbanButtons = document.querySelectorAll('.btn-unban-single');
            unbanButtons.forEach(button => {
                button.addEventListener('click', (e) => {
                    e.stopPropagation();
                    const projectId = button.getAttribute('data-project-id');
                    
                    if (confirm('Are you sure you want to unban credential for project: ' + projectId + '?')) {
                        unbanCredentials([projectId]);
                    }
                });
            });

            // Single delete button listeners
            const deleteButtons = document.querySelectorAll('.btn-delete-single');
            deleteButtons.forEach(button => {
                button.addEventListener('click', (e) => {
                    e.stopPropagation(); // Prevent card click
                    const projectId = button.getAttribute('data-project-id');
                    
                    if (confirm('Are you sure you want to delete credential for project: ' + projectId + '?')) {
                        deleteSingleCredential(projectId);
                    }
                });
            });

            // Select all button
            selectAllBtn.addEventListener('click', () => {
                const allCheckboxes = document.querySelectorAll('.credential-checkbox');
                const allSelected = allCheckboxes.length > 0 && selectedProjects.size === allCheckboxes.length;
                
                if (allSelected) {
                    deselectAll();
                } else {
                    selectAll();
                }
            });

            // Bulk ban button
            bulkBanBtn.addEventListener('click', () => {
                const projectIds = Array.from(selectedProjects);
                if (projectIds.length === 0) return;
                
                if (confirm('Are you sure you want to ban ' + projectIds.length + ' credential(s)?')) {
                    banCredentials(projectIds);
                }
            });

            // Bulk unban button
            bulkUnbanBtn.addEventListener('click', () => {
                const projectIds = Array.from(selectedProjects);
                if (projectIds.length === 0) return;
                
                if (confirm('Are you sure you want to unban ' + projectIds.length + ' credential(s)?')) {
                    unbanCredentials(projectIds);
                }
            });

            // Bulk delete button
            bulkDeleteBtn.addEventListener('click', () => {
                const projectIds = Array.from(selectedProjects);
                if (projectIds.length === 0) return;
                
                // Show confirmation
                const confirmMsg = 'Are you sure you want to delete ' + projectIds.length + ' credential(s)?';
                if (confirm(confirmMsg)) {
                    bulkDeleteCredentials(projectIds);
                }
            });


        });

        // Handle page visibility for OAuth callback
        // If user returns from OAuth, hide any lingering loading states
        document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'visible') {
                loading.hide();
            }
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            // ESC key closes toast
            if (e.key === 'Escape') {
                if (toast.element.classList.contains('active')) {
                    toast.hide();
                }
            }
        });
    </script>
</body>
</html>`

// oauthCallbackStreamTemplate is the embedded HTML template for the streaming OAuth callback page
var oauthCallbackStreamTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OAuth Flow - Gemini CLI to API Proxy</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }

        .callback-container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
            padding: 40px;
            width: 100%;
            max-width: 600px;
        }

        .status-icon {
            font-size: 64px;
            margin-bottom: 20px;
            text-align: center;
        }

        .status-icon.processing {
            color: #2196f3;
            animation: pulse 1.5s ease-in-out infinite;
        }

        .status-icon.success {
            color: #4caf50;
        }

        .status-icon.warning {
            color: #ff9800;
        }

        .status-icon.error {
            color: #f44336;
        }

        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }

        h1 {
            color: #333;
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 16px;
            text-align: center;
        }

        .progress-bar-container {
            width: 100%;
            height: 8px;
            background: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin: 20px 0;
        }

        .progress-bar {
            height: 100%;
            background: linear-gradient(90deg, #667eea 0%, #764ba2 100%);
            width: 0%;
            transition: width 0.3s ease-out;
        }

        .log-container {
            max-height: 400px;
            overflow-y: auto;
            background: #f5f5f5;
            border-radius: 8px;
            padding: 16px;
            margin: 20px 0;
            text-align: left;
        }

        .log-entry {
            padding: 8px 0;
            font-size: 14px;
            line-height: 1.6;
            display: flex;
            align-items: flex-start;
            gap: 8px;
            animation: slideIn 0.3s ease-out;
        }

        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateX(-10px);
            }
            to {
                opacity: 1;
                transform: translateX(0);
            }
        }

        .log-entry .icon {
            flex-shrink: 0;
            font-size: 16px;
        }

        .log-entry.progress {
            color: #2196f3;
        }

        .log-entry.success {
            color: #4caf50;
        }

        .log-entry.warning {
            color: #ff9800;
        }

        .log-entry.error {
            color: #f44336;
        }

        .log-entry.complete {
            color: #333;
            font-weight: 600;
            padding-top: 16px;
            border-top: 2px solid #e0e0e0;
            margin-top: 8px;
        }

        .summary {
            text-align: center;
            margin-top: 20px;
            padding: 20px;
            background: #f9f9f9;
            border-radius: 8px;
            display: none;
        }

        .summary.visible {
            display: block;
        }

        .summary-text {
            color: #333;
            font-size: 16px;
            line-height: 1.6;
            margin-bottom: 20px;
        }

        .btn-dashboard {
            display: inline-block;
            padding: 12px 32px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .btn-dashboard:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(102, 126, 234, 0.4);
        }

        @media (max-width: 480px) {
            .callback-container {
                padding: 30px 20px;
            }
            h1 {
                font-size: 20px;
            }
        }
    </style>
</head>
<body>
    <div class="callback-container">
        <div class="status-icon processing" id="statusIcon">⏳</div>
        <h1 id="statusTitle">Processing OAuth Flow...</h1>
        
        <div class="progress-bar-container">
            <div class="progress-bar" id="progressBar"></div>
        </div>

        <div class="log-container" id="logContainer">
            <div class="log-entry progress">
                <span class="icon">🔄</span>
                <span>Connecting to server...</span>
            </div>
        </div>

        <div class="summary" id="summary">
            <div class="summary-text" id="summaryText"></div>
            <a href="/" class="btn-dashboard">Go to Dashboard</a>
        </div>
    </div>

    <script>
        const statusIcon = document.getElementById('statusIcon');
        const statusTitle = document.getElementById('statusTitle');
        const progressBar = document.getElementById('progressBar');
        const logContainer = document.getElementById('logContainer');
        const summary = document.getElementById('summary');
        const summaryText = document.getElementById('summaryText');

        let progress = 0;
        let totalSteps = 0;
        let completedSteps = 0;

        function addLogEntry(type, message) {
            const entry = document.createElement('div');
            entry.className = 'log-entry ' + type;
            
            let icon = '•';
            if (type === 'progress') icon = '🔄';
            else if (type === 'success') icon = '✓';
            else if (type === 'warning') icon = '⚠️';
            else if (type === 'error') icon = '✗';
            else if (type === 'complete') icon = '🎉';
            
            entry.innerHTML = '<span class="icon">' + icon + '</span><span>' + message + '</span>';
            logContainer.appendChild(entry);
            
            // Auto-scroll to bottom
            logContainer.scrollTop = logContainer.scrollHeight;
        }

        function updateProgress(percent) {
            progressBar.style.width = percent + '%';
        }

        function setFinalStatus(status) {
            statusIcon.className = 'status-icon ' + status;
            
            if (status === 'success') {
                statusIcon.textContent = '✓';
                statusTitle.textContent = 'OAuth Flow Completed!';
            } else if (status === 'warning') {
                statusIcon.textContent = '⚠️';
                statusTitle.textContent = 'OAuth Flow Completed with Warnings';
            } else if (status === 'error') {
                statusIcon.textContent = '✗';
                statusTitle.textContent = 'OAuth Flow Failed';
            }
            
            summary.classList.add('visible');
        }

        // Connect to Server-Sent Events stream
        const eventSource = new EventSource(window.location.href);

        eventSource.addEventListener('progress', function(e) {
            addLogEntry('progress', e.data);
            completedSteps++;
            updateProgress(Math.min(90, (completedSteps / Math.max(totalSteps, 10)) * 90));
        });

        eventSource.addEventListener('success', function(e) {
            addLogEntry('success', e.data);
            completedSteps++;
            updateProgress(Math.min(90, (completedSteps / Math.max(totalSteps, 10)) * 90));
        });

        eventSource.addEventListener('warning', function(e) {
            addLogEntry('warning', e.data);
        });

        eventSource.addEventListener('error', function(e) {
            if (e.data) {
                addLogEntry('error', e.data);
            }
        });

        eventSource.addEventListener('complete', function(e) {
            addLogEntry('complete', e.data);
            summaryText.textContent = e.data;
            updateProgress(100);
        });

        eventSource.addEventListener('done', function(e) {
            const status = e.data; // 'success', 'warning', or 'error'
            setFinalStatus(status);
            eventSource.close();
        });

        eventSource.onerror = function(e) {
            if (eventSource.readyState === EventSource.CLOSED) {
                // Connection closed normally
                return;
            }
            
            // Connection error
            addLogEntry('error', 'Connection error. Please check your network and try again.');
            setFinalStatus('error');
            summaryText.textContent = 'Connection error occurred. Please try again.';
            eventSource.close();
        };

        // Estimate total steps based on typical flow
        totalSteps = 10; // Will be adjusted as we receive events
    </script>
</body>
</html>`

// oauthCallbackTemplate is the embedded HTML template for the OAuth callback progress page
var oauthCallbackTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OAuth Flow - Gemini CLI to API Proxy</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }

        .callback-container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
            padding: 40px;
            width: 100%;
            max-width: 500px;
            text-align: center;
        }

        .status-icon {
            font-size: 64px;
            margin-bottom: 20px;
        }

        .status-icon.success {
            color: #4caf50;
        }

        .status-icon.error {
            color: #f44336;
        }

        .status-icon.processing {
            color: #2196f3;
            animation: pulse 1.5s ease-in-out infinite;
        }

        @keyframes pulse {
            0%, 100% {
                opacity: 1;
            }
            50% {
                opacity: 0.5;
            }
        }

        h1 {
            color: #333;
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 16px;
        }

        .message {
            color: #666;
            font-size: 16px;
            line-height: 1.6;
            margin-bottom: 30px;
        }

        .progress-list {
            text-align: left;
            margin: 30px 0;
            padding: 20px;
            background: #f5f5f5;
            border-radius: 8px;
        }

        .progress-item {
            display: flex;
            align-items: center;
            padding: 10px 0;
            color: #333;
            font-size: 14px;
        }

        .progress-item .icon {
            margin-right: 12px;
            font-size: 18px;
        }

        .progress-item.complete .icon {
            color: #4caf50;
        }

        .progress-item.processing .icon {
            color: #2196f3;
        }

        .progress-item.pending .icon {
            color: #ccc;
        }

        .btn-dashboard {
            display: inline-block;
            padding: 12px 32px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .btn-dashboard:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(102, 126, 234, 0.4);
        }

        .btn-retry {
            display: inline-block;
            padding: 12px 32px;
            background: #f44336;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            margin-right: 10px;
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .btn-retry:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(244, 67, 54, 0.4);
        }

        /* Progress indicator */
        .progress-bar-container {
            width: 100%;
            height: 8px;
            background: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin: 20px 0;
        }

        .progress-bar {
            height: 100%;
            background: linear-gradient(90deg, #667eea 0%, #764ba2 100%);
            width: 0%;
            transition: width 0.3s ease-out;
            animation: progressPulse 1.5s ease-in-out infinite;
        }

        @keyframes progressPulse {
            0%, 100% {
                opacity: 1;
            }
            50% {
                opacity: 0.7;
            }
        }

        .detail-text {
            color: #999;
            font-size: 13px;
            margin-top: 10px;
            font-style: italic;
        }

        @media (max-width: 480px) {
            .callback-container {
                padding: 30px 20px;
            }

            h1 {
                font-size: 20px;
            }

            .message {
                font-size: 14px;
            }
        }
    </style>
</head>
<body>
    <div class="callback-container">
        {{if eq .Status "success"}}
        <div class="status-icon success">✓</div>
        <h1>Authorization Successful!</h1>
        {{else if eq .Status "error"}}
        <div class="status-icon error">✗</div>
        <h1>Authorization Failed</h1>
        {{else}}
        <div class="status-icon processing">⏳</div>
        <h1>Processing OAuth Flow...</h1>
        <div class="progress-bar-container">
            <div class="progress-bar" id="progressBar"></div>
        </div>
        {{end}}

        <div class="message">
            {{.Message}}
        </div>

        {{if eq .Status "processing"}}
        <div class="detail-text" id="detailText">Please wait while we set up your credentials...</div>
        {{end}}

        {{if eq .Status "success"}}
        <a href="/" class="btn-dashboard">Go to Dashboard</a>
        {{else if eq .Status "error"}}
        <a href="/dashboard/oauth/start" class="btn-retry">Try Again</a>
        <a href="/" class="btn-dashboard">Go to Dashboard</a>
        {{end}}
    </div>

    {{if eq .Status "processing"}}
    <script>
        // Simulate progress for better UX during OAuth processing
        let progress = 0;
        const progressBar = document.getElementById('progressBar');
        const detailText = document.getElementById('detailText');
        
        const steps = [
            { progress: 20, text: 'Exchanging authorization code...' },
            { progress: 40, text: 'Discovering Google Cloud projects...' },
            { progress: 60, text: 'Enabling required APIs...' },
            { progress: 80, text: 'Saving credentials...' },
            { progress: 95, text: 'Finalizing setup...' }
        ];
        
        let currentStep = 0;
        
        function updateProgress() {
            if (currentStep < steps.length) {
                const step = steps[currentStep];
                progressBar.style.width = step.progress + '%';
                detailText.textContent = step.text;
                currentStep++;
                
                // Random delay between 800ms and 1500ms for realistic feel
                const delay = 800 + Math.random() * 700;
                setTimeout(updateProgress, delay);
            }
        }
        
        // Start progress animation after a brief delay
        setTimeout(updateProgress, 500);
    </script>
    {{end}}
</body>
</html>`

// RenderLogin renders the login page with an optional error message
func RenderLogin(w http.ResponseWriter, errorMsg string, lang i18n.Language) {
	log.Printf("[DEBUG] Rendering login page (error: %v, lang: %s)", errorMsg != "", lang)

	tmpl, err := template.New("login").Parse(loginTemplate)
	if err != nil {
		log.Printf("[ERROR] Failed to parse login template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		ErrorMessage string
		Lang         string
		T            map[string]string
	}{
		ErrorMessage: errorMsg,
		Lang:         string(lang),
		T:            i18n.GetAllTranslations(lang),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[ERROR] Failed to execute login template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[DEBUG] Login page rendered successfully")
}

// RenderDashboard renders the main dashboard page with credential information
func RenderDashboard(w http.ResponseWriter, credentials []CredentialInfo, lang i18n.Language) {
	log.Printf("[DEBUG] Rendering dashboard page with %d credentials (lang: %s)", len(credentials), lang)

	tmpl, err := template.New("dashboard").Parse(dashboardTemplate)
	if err != nil {
		log.Printf("[ERROR] Failed to parse dashboard template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Credentials []CredentialInfo
		Lang        string
		T           map[string]string
	}{
		Credentials: credentials,
		Lang:        string(lang),
		T:           i18n.GetAllTranslations(lang),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[ERROR] Failed to execute dashboard template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[DEBUG] Dashboard page rendered successfully")
}

// RenderOAuthCallback renders the OAuth callback progress page
func RenderOAuthCallback(w http.ResponseWriter, status string, message string) {
	log.Printf("[INFO] Rendering OAuth callback page (status: %s)", status)

	tmpl, err := template.New("oauthCallback").Parse(oauthCallbackTemplate)
	if err != nil {
		log.Printf("[ERROR] Failed to parse OAuth callback template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Status  string
		Message string
	}{
		Status:  status,
		Message: message,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[ERROR] Failed to execute OAuth callback template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[DEBUG] OAuth callback page rendered successfully")
}

// RenderOAuthCallbackStream renders the streaming OAuth callback page
func RenderOAuthCallbackStream(w http.ResponseWriter) {
	log.Printf("[INFO] Rendering streaming OAuth callback page")

	tmpl, err := template.New("oauthCallbackStream").Parse(oauthCallbackStreamTemplate)
	if err != nil {
		log.Printf("[ERROR] Failed to parse OAuth callback stream template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		log.Printf("[ERROR] Failed to execute OAuth callback stream template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[DEBUG] OAuth callback stream page rendered successfully")
}
