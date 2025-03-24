// Theme management for initial page load and system theme changes
document.addEventListener('DOMContentLoaded', () => {
    const themeToggle = document.getElementById('theme-toggle');
    if (!themeToggle) return;
    
    const html = document.documentElement;
    const currentTheme = html.getAttribute('data-theme');

    // Apply theme on initial page load
    applyTheme(currentTheme);

    // Handle theme changed event from server
    document.addEventListener('themeChanged', (e) => {
        if (e.detail && e.detail.theme) {
            applyTheme(e.detail.theme);
        }
    });
    
    // Add an additional direct click handler to provide immediate feedback
    themeToggle.addEventListener('click', () => {
        // Get the current theme
        const currentTheme = html.getAttribute('data-theme');
        let newTheme;
        
        // Calculate the next theme in the cycle
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
        
        // Apply the new theme immediately for faster visual feedback
        // The server response will still update the theme officially
        applyTheme(newTheme);
    });

    // Helper function to apply theme styles and update icons
    function applyTheme(theme) {
        // Update theme attribute
        html.setAttribute('data-theme', theme);
        
        // Update the theme toggle icons
        updateThemeIcons(theme);
        
        // For system theme, apply the OS preference
        if (theme === 'system') {
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            html.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
        }
        
        // Set cookie for theme persistence (fallback for client-side)
        document.cookie = `theme=${theme};path=/;max-age=${365 * 24 * 60 * 60}`;
    }

    // Helper function to update the theme icons based on theme
    function updateThemeIcons(theme) {
        const sunIcon = themeToggle.querySelector('.sun-icon');
        const moonIcon = themeToggle.querySelector('.moon-icon');
        
        // Hide both icons initially
        sunIcon.style.display = 'none';
        moonIcon.style.display = 'none';
        
        // Show the appropriate icon based on the theme
        if (theme === 'light') {
            sunIcon.style.display = 'block';
        } else if (theme === 'dark') {
            moonIcon.style.display = 'block';
        } else if (theme === 'system') {
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            if (prefersDark) {
                moonIcon.style.display = 'block';
            } else {
                sunIcon.style.display = 'block';
            }
        }
    }

    // Listen for system theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        const currentTheme = html.getAttribute('data-theme');
        if (currentTheme === 'system') {
            const prefersDark = e.matches;
            html.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
            updateThemeIcons('system');
        }
    });
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
        document.body.classList.toggle('menu-open');
    });

    // Close menu when clicking outside
    document.addEventListener('click', (e) => {
        if (!navLinks.contains(e.target) && !menuToggle.contains(e.target)) {
            navLinks.classList.remove('active');
            document.body.classList.remove('menu-open');
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
            document.body.classList.remove('menu-open');
        }
    });
}); 