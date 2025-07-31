document.getElementById('loginForm').addEventListener('submit', function(e) {
    const username = document.getElementById('username').value.trim();
    const errorDiv = document.getElementById('error');
    
    // Validate username
    if (!username || username.length < 2) {
        e.preventDefault();
        errorDiv.style.display = 'block';
        return;
    }
    
    // Clean username (remove special characters, limit length)
    const cleanUsername = username.replace(/[^a-zA-Z0-9_-]/g, '').substring(0, 20);
    
    if (cleanUsername.length < 2) {
        e.preventDefault();
        errorDiv.style.display = 'block';
        return;
    }
    
    // Update the form input with cleaned username
    document.getElementById('username').value = cleanUsername;
});

// Handle Enter key
document.getElementById('username').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        document.getElementById('loginForm').dispatchEvent(new Event('submit'));
    }
}); 