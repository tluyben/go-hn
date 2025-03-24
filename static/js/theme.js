// Theme management
const themeToggle = document.getElementById('theme-toggle');
const html = document.documentElement;

// Initialize settings from localStorage or use defaults
const settings = JSON.parse(localStorage.getItem('settings')) || {
    theme: 'system'
};

// Apply initial theme
applyTheme(settings.theme);

// Theme toggle click handler
themeToggle.addEventListener('click', () => {
    const currentTheme = html.getAttribute('data-theme');
    let newTheme;
    
    switch (currentTheme) {
        case 'light':
            newTheme = 'dark';
            break;
        case 'dark':
            newTheme = 'system';
            break;
        default:
            newTheme = 'light';
    }
    
    applyTheme(newTheme);
    settings.theme = newTheme;
    localStorage.setItem('settings', JSON.stringify(settings));
});

// Apply theme function
function applyTheme(theme) {
    html.setAttribute('data-theme', theme);
    
    // Update theme toggle button state
    const sunIcon = themeToggle.querySelector('.sun-icon');
    const moonIcon = themeToggle.querySelector('.moon-icon');
    
    sunIcon.style.display = theme === 'light' ? 'block' : 'none';
    moonIcon.style.display = theme === 'dark' ? 'block' : 'none';
    
    // Handle system theme preference
    if (theme === 'system') {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        html.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    }
}

// Listen for system theme changes
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    if (settings.theme === 'system') {
        applyTheme('system');
    }
});

// Mobile menu functionality
document.addEventListener('DOMContentLoaded', () => {
    const menuToggle = document.querySelector('.menu-toggle');
    const navLinks = document.querySelector('.nav-links');
    
    if (!menuToggle || !navLinks) return;

    // Toggle menu
    menuToggle.addEventListener('click', (e) => {
        e.stopPropagation();
        navLinks.classList.toggle('active');
    });

    // Close menu when clicking outside
    document.addEventListener('click', (e) => {
        if (!navLinks.contains(e.target) && !menuToggle.contains(e.target)) {
            navLinks.classList.remove('active');
        }
    });

    // Prevent menu from closing when clicking inside it
    navLinks.addEventListener('click', (e) => {
        e.stopPropagation();
    });

    // Close menu when window is resized above mobile breakpoint
    window.addEventListener('resize', () => {
        if (window.innerWidth > 768) {
            navLinks.classList.remove('active');
        }
    });
}); 