document.addEventListener('DOMContentLoaded', function() {
  const menuToggle = document.querySelector('.menu-toggle');
  const headerMenu = document.querySelector('.header-menu');
  
  if (menuToggle && headerMenu) {
    menuToggle.addEventListener('click', function() {
      const isExpanded = this.getAttribute('aria-expanded') === 'true';
      this.setAttribute('aria-expanded', !isExpanded);
      headerMenu.classList.toggle('is-open');
      document.body.style.overflow = !isExpanded ? 'hidden' : '';
    });
    
    document.addEventListener('click', function(e) {
      if (!e.target.closest('.header-wrapper')) {
        menuToggle.setAttribute('aria-expanded', 'false');
        headerMenu.classList.remove('is-open');
        document.body.style.overflow = '';
      }
    });
    
    window.addEventListener('resize', function() {
      if (window.innerWidth > 768) {
        menuToggle.setAttribute('aria-expanded', 'false');
        headerMenu.classList.remove('is-open');
        document.body.style.overflow = '';
      }
    });
  }
  
  const langToggle = document.querySelector('.lang-toggle');
  const langDropdown = document.querySelector('.lang-dropdown');
  
  if (langToggle && langDropdown && window.innerWidth <= 768) {
    langToggle.addEventListener('click', function(e) {
      e.stopPropagation();
      langDropdown.classList.toggle('is-open');
    });
  }
  
  const themeToggle = document.querySelector('.theme-toggle');
  const htmlElement = document.documentElement;
  const currentTheme = localStorage.getItem('theme') || 'light';
  
  htmlElement.setAttribute('data-theme', currentTheme);
  
  if (themeToggle) {
    themeToggle.addEventListener('click', function() {
      const theme = htmlElement.getAttribute('data-theme');
      const newTheme = theme === 'dark' ? 'light' : 'dark';
      htmlElement.setAttribute('data-theme', newTheme);
      localStorage.setItem('theme', newTheme);
    });
  }
});
