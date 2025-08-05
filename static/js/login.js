document.addEventListener('DOMContentLoaded', function() {
    // Get DOM elements
    const errorDiv = document.getElementById('error');
    const passkeyLoginBtn = document.getElementById('passkeyLoginBtn');
    const passkeyRegisterBtn = document.getElementById('passkeyRegisterBtn');
    const registrationModal = document.getElementById('registrationModal');
    const modalClose = document.getElementById('modalClose');
    const cancelRegisterBtn = document.getElementById('cancelRegisterBtn');
    const confirmRegisterBtn = document.getElementById('confirmRegisterBtn');
    const usernameInput = document.getElementById('username');
    const modalError = document.getElementById('modalError');
    const successNotification = document.getElementById('successNotification');
    const notificationClose = document.getElementById('notificationClose');

    // WebAuthn variables
    let webauthnClient;
    let isWebAuthnSupported = false;

    // Initialize WebAuthn
    function initializeWebAuthn() {
        if (window.WebAuthnClient) {
            webauthnClient = new WebAuthnClient();
            isWebAuthnSupported = webauthnClient.isSupported();
            
            if (isWebAuthnSupported) {
                passkeyLoginBtn.disabled = false;
                passkeyRegisterBtn.disabled = false;
            } else {
                showError('Passkeys are not supported in your browser. Please use a modern browser with WebAuthn support.');
            }
        }
    }

    // Show error message
    function showError(message) {
        errorDiv.textContent = message;
        errorDiv.classList.add('show');
        document.body.classList.remove('loading');
    }

    // Hide error message
    function hideError() {
        errorDiv.classList.remove('show');
    }

    // Show modal error message
    function showModalError(message) {
        modalError.innerHTML = message; // Use innerHTML to allow HTML line breaks
        modalError.classList.add('show');
    }

    // Hide modal error message
    function hideModalError() {
        modalError.classList.remove('show');
    }

    // Show loading state
    function showLoading(button) {
        button.classList.add('loading');
        hideError();
        hideModalError();
    }

    // Hide loading state
    function hideLoading(button) {
        button.classList.remove('loading');
    }

    // Show success notification
    function showSuccessNotification() {
        successNotification.classList.add('show');
        
        // Auto-hide after 8 seconds
        setTimeout(() => {
            hideSuccessNotification();
        }, 8000);
    }

    // Hide success notification
    function hideSuccessNotification() {
        successNotification.classList.remove('show');
    }

    // Show modal
    function showModal() {
        registrationModal.classList.add('show');
        usernameInput.focus();
        hideModalError();
        usernameInput.value = '';
    }

    // Hide modal
    function hideModal() {
        registrationModal.classList.remove('show');
        hideModalError();
        usernameInput.value = '';
        
        // Remove the setTimeout that sets display: none
        // setTimeout(() => {
        //     registrationModal.style.display = 'none';
        // }, 300); // Match the CSS transition duration
    }

    // Handle passkey registration
    async function handlePasskeyRegistration() {
        const username = usernameInput.value.trim();
        
        if (!username) {
            showModalError('Please enter a username for registration');
            return;
        }

        if (!isWebAuthnSupported) {
            showModalError('Passkeys are not supported in your browser');
        return;
    }
    
        showLoading(confirmRegisterBtn);

        try {
            await webauthnClient.register(username);
            hideLoading(confirmRegisterBtn);
            hideModal();
            
            // Show success notification
            showSuccessNotification();
            
            // Don't redirect automatically - let user log in with their new passkey
            // setTimeout(() => {
            //     window.location.href = '/';
            // }, 1000);
            
        } catch (error) {
            hideLoading(confirmRegisterBtn);
            // Don't log technical errors - they're handled gracefully in the UI
            
            // Check for different error message formats
            const errorMessage = error.message || error.toString();
            
            if (errorMessage.includes('409') || errorMessage.includes('Conflict') || errorMessage.includes('User already exists')) {
                showModalError('User already exists.<br>Please choose a different username.');
            } else if (errorMessage.includes('NotAllowedError')) {
                showModalError('Registration was cancelled or not supported by your device.');
            } else if (errorMessage.includes('NotSupportedError')) {
                showModalError('Your device does not support passkeys.');
            } else if (errorMessage.includes('InvalidStateError')) {
                showModalError('A passkey already exists for this account.');
            } else if (errorMessage.includes('Registration failed')) {
                showModalError('Registration failed. Please try again.');
            } else {
                showModalError('Passkey registration failed. Please try again.');
            }
        }
    }

    // Handle passkey login
    async function handlePasskeyLogin() {
        if (!isWebAuthnSupported) {
            showError('Passkeys are not supported in your browser');
        return;
        }

        showLoading(passkeyLoginBtn);

        try {
            const result = await webauthnClient.login();
            hideLoading(passkeyLoginBtn);
            
            // Check if this is a CLI authentication request
            const urlParams = new URLSearchParams(window.location.search);
            const isCLI = urlParams.get('cli') === 'true';
            
            if (isCLI) {
                // For CLI authentication, handle the redirect response
                console.log('CLI authentication successful');
                
                // If we got a redirect response, follow it
                if (result && result.status === 'redirect') {
                    console.log('Following redirect to:', result.redirect);
                    window.location.href = result.redirect;
                }
            } else {
                // Redirect to chat after successful login
                setTimeout(() => {
                    window.location.href = '/';
                }, 1000);
            }
            
        } catch (error) {
            hideLoading(passkeyLoginBtn);
            // Don't log technical errors - they're handled gracefully in the UI
            
            // Check for different error message formats
            const errorMessage = error.message || error.toString();
            
            if (errorMessage.includes('404') || errorMessage.includes('Not Found')) {
                showError('No passkey found. Please register first.');
            } else if (errorMessage.includes('NotAllowedError')) {
                showError('Login was cancelled or not supported by your device.');
            } else if (errorMessage.includes('NotSupportedError')) {
                showError('Your device does not support passkeys.');
            } else if (errorMessage.includes('InvalidStateError')) {
                showError('No passkey found for this account.');
            } else if (errorMessage.includes('Login failed')) {
                showError('Login failed. Please try again.');
            } else {
                showError('Passkey login failed. Please try again.');
            }
        }
    }

    // Event listeners
    passkeyLoginBtn.addEventListener('click', handlePasskeyLogin);
    passkeyRegisterBtn.addEventListener('click', function() {
        showModal();
    });
    
    // Fallback test - add click listener regardless of WebAuthn support
    passkeyRegisterBtn.addEventListener('click', function() {
    });
    
    // Modal event listeners
    modalClose.addEventListener('click', hideModal);
    cancelRegisterBtn.addEventListener('click', hideModal);
    confirmRegisterBtn.addEventListener('click', handlePasskeyRegistration);
    
    // Notification event listeners
    notificationClose.addEventListener('click', hideSuccessNotification);
    
    // Close modal when clicking outside
    registrationModal.addEventListener('click', function(e) {
        if (e.target === registrationModal) {
            hideModal();
        }
    });
    
    // Handle Enter key in modal
    usernameInput.addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
            e.preventDefault();
            handlePasskeyRegistration();
        }
    });
    
    // Clear modal error when typing
    usernameInput.addEventListener('input', hideModalError);

    // Initialize WebAuthn when page loads
    initializeWebAuthn();
    
    // Reset page state on load
    function resetPageState() {
        hideModal();
        hideError();
        document.body.classList.remove('loading', 'success');
    }
    
    // Call reset on page load
    resetPageState();
}); 