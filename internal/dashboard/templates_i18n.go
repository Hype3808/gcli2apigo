package dashboard

// This file contains i18n-enabled template snippets

// Language switcher HTML component
const languageSwitcherHTML = `
<div class="language-switcher">
    <button class="lang-btn" id="langBtn" title="{{.T.language.switch}}">
        <span class="lang-icon">🌐</span>
        <span class="lang-text" id="currentLang">{{if eq .Lang "zh"}}中文{{else}}EN{{end}}</span>
    </button>
    <div class="lang-menu" id="langMenu">
        <button class="lang-option" data-lang="zh">
            <span class="lang-flag">🇨🇳</span>
            <span>中文</span>
        </button>
        <button class="lang-option" data-lang="en">
            <span class="lang-flag">🇺🇸</span>
            <span>English</span>
        </button>
    </div>
</div>
`

// Language switcher CSS
const languageSwitcherCSS = `
.language-switcher {
    position: relative;
    display: inline-block;
}

.lang-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: #2a2a2a;
    color: #e0e0e0;
    border: 1px solid #3a3a3a;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
}

.lang-btn:hover {
    background: #3a3a3a;
    border-color: #4a4a4a;
}

.lang-icon {
    font-size: 16px;
}

.lang-text {
    font-weight: 600;
}

.lang-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    background: #1a1a1a;
    border: 1px solid #3a3a3a;
    border-radius: 8px;
    min-width: 140px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    opacity: 0;
    visibility: hidden;
    transform: translateY(-10px);
    transition: all 0.2s;
    z-index: 1000;
    overflow: hidden;
}

.language-switcher.active .lang-menu {
    opacity: 1;
    visibility: visible;
    transform: translateY(0);
}

.lang-option {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 12px 16px;
    background: transparent;
    color: #e0e0e0;
    border: none;
    cursor: pointer;
    transition: all 0.2s;
    font-size: 14px;
}

.lang-option:hover {
    background: rgba(139, 92, 246, 0.1);
}

.lang-option.active {
    background: rgba(139, 92, 246, 0.2);
    color: #8b5cf6;
}

.lang-flag {
    font-size: 18px;
}

@media (max-width: 768px) {
    .lang-btn {
        padding: 6px 12px;
        font-size: 13px;
    }
    
    .lang-text {
        display: none;
    }
}
`

// Language switcher JavaScript
const languageSwitcherJS = `
// Language switcher functionality
const langSwitcher = document.querySelector('.language-switcher');
const langBtn = document.getElementById('langBtn');
const langMenu = document.getElementById('langMenu');
const langOptions = document.querySelectorAll('.lang-option');
const currentLangText = document.getElementById('currentLang');

if (langBtn) {
    langBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        langSwitcher.classList.toggle('active');
    });
}

// Close language menu when clicking outside
document.addEventListener('click', (e) => {
    if (langSwitcher && !langSwitcher.contains(e.target)) {
        langSwitcher.classList.remove('active');
    }
});

// Handle language selection
langOptions.forEach(option => {
    option.addEventListener('click', async () => {
        const newLang = option.getAttribute('data-lang');
        
        try {
            const response = await fetch('/dashboard/api/language', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ language: newLang })
            });
            
            if (response.ok) {
                // Reload page to apply new language
                window.location.reload();
            }
        } catch (error) {
            console.error('Failed to change language:', error);
        }
    });
});
`
