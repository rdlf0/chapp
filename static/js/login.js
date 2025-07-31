// Handle Enter key
document.getElementById('username').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        // Let the form submit naturally
    }
});

// Add loading state when form is about to submit
document.getElementById('loginForm').addEventListener('submit', function(e) {
    const username = document.getElementById('username').value.trim();
    
    // Add loading state but don't prevent submission
    const submitBtn = document.querySelector('.submit-btn');
    submitBtn.disabled = true;
    submitBtn.textContent = 'Joining...';
    
    // Don't call e.preventDefault() - let the form submit naturally
}); 