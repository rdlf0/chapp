// WebAuthn Client-side Implementation
class WebAuthnClient {
    constructor() {
        this.baseURL = window.location.origin;
    }

    // Check if WebAuthn is supported
    isSupported() {
        // Check if PublicKeyCredential exists
        if (!window.PublicKeyCredential) {
            return false;
        }
        
        // Check if user verifying platform authenticator is available
        if (!PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable) {
            return false;
        }
        
        // Check if conditional mediation is available (optional for some browsers)
        if (PublicKeyCredential.isConditionalMediationAvailable) {
            // Conditional mediation is available
        } else {
            // Conditional mediation not available, but continuing...
        }
        
        return true; // If we get here, basic WebAuthn is supported
    }

    // Convert base64url to ArrayBuffer
    base64URLToArrayBuffer(base64URL) {
        const padding = '='.repeat((4 - base64URL.length % 4) % 4);
        const base64 = (base64URL + padding)
            .replace(/-/g, '+')
            .replace(/_/g, '/');

        const rawData = window.atob(base64);
        const outputArray = new Uint8Array(rawData.length);

        for (let i = 0; i < rawData.length; ++i) {
            outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray.buffer;
    }

    // Convert ArrayBuffer to base64url
    arrayBufferToBase64URL(buffer) {
        const bytes = new Uint8Array(buffer);
        let binary = '';
        for (let i = 0; i < bytes.byteLength; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return window.btoa(binary)
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=/g, '');
    }

    // Begin passkey registration
    async beginRegistration(username) {
        try {
            const response = await fetch(`${this.baseURL}/webauthn/begin-registration`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username })
            });

            if (!response.ok) {
                // Handle specific error cases gracefully
                if (response.status === 409) {
                    const error = new Error('User already exists');
                    error.status = 409;
                    throw error;
                }
                throw new Error(`Registration failed: ${response.status}`);
            }

            const options = await response.json();
            
            // Convert challenge from base64url to ArrayBuffer
            options.publicKey.challenge = this.base64URLToArrayBuffer(options.publicKey.challenge);
            
            // Convert user ID from base64url to ArrayBuffer
            if (options.publicKey.user) {
                options.publicKey.user.id = this.base64URLToArrayBuffer(options.publicKey.user.id);
            }

            // Create the credential
            const credential = await navigator.credentials.create({
                publicKey: options.publicKey
            });

            return credential;
        } catch (error) {
            // Don't log technical errors - they're handled gracefully in the UI
            throw error;
        }
    }

    // Finish passkey registration
    async finishRegistration(username, credential) {
        try {
            const response = await fetch(`${this.baseURL}/webauthn/finish-registration`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    id: credential.id,
                    rawId: this.arrayBufferToBase64URL(credential.rawId),
                    username: username, // Include username in request body
                    response: {
                        attestationObject: this.arrayBufferToBase64URL(credential.response.attestationObject),
                        clientDataJSON: this.arrayBufferToBase64URL(credential.response.clientDataJSON),
                    },
                    type: credential.type
                })
            });

            if (!response.ok) {
                throw new Error(`Registration completion failed: ${response.status}`);
            }

            return await response.json();
        } catch (error) {
            // Don't log technical errors - they're handled gracefully in the UI
            throw error;
        }
    }

    // Begin passkey authentication
    async beginLogin() {
        try {
            		const response = await fetch(`${this.baseURL}/webauthn/begin-login`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({}) // No username needed for login
            });

            if (!response.ok) {
                throw new Error(`Login failed: ${response.status}`);
            }

            const options = await response.json();
            
            // Convert challenge from base64url to ArrayBuffer
            options.publicKey.challenge = this.base64URLToArrayBuffer(options.publicKey.challenge);

            // Get the credential
            const assertion = await navigator.credentials.get({
                publicKey: options.publicKey
            });

            return assertion;
        } catch (error) {
            // Don't log technical errors - they're handled gracefully in the UI
            throw error;
        }
    }

    // Finish passkey authentication
    async finishLogin(assertion) {
        try {
            		const response = await fetch(`${this.baseURL}/webauthn/finish-login`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    id: assertion.id,
                    rawId: this.arrayBufferToBase64URL(assertion.rawId),
                    response: {
                        authenticatorData: this.arrayBufferToBase64URL(assertion.response.authenticatorData),
                        clientDataJSON: this.arrayBufferToBase64URL(assertion.response.clientDataJSON),
                        signature: this.arrayBufferToBase64URL(assertion.response.signature),
                    },
                    type: assertion.type
                })
            });

            if (!response.ok) {
                throw new Error(`Login completion failed: ${response.status}`);
            }



            return await response.json();
        } catch (error) {
            // Don't log technical errors - they're handled gracefully in the UI
            throw error;
        }
    }

    // Complete registration flow
    async register(username) {
        try {
            const credential = await this.beginRegistration(username);
            const result = await this.finishRegistration(username, credential);
            return result;
        } catch (error) {
            throw error;
        }
    }

    // Complete login flow
    async login() {
        try {
            const assertion = await this.beginLogin();
            const result = await this.finishLogin(assertion);
            return result;
        } catch (error) {
            throw error;
        }
    }
}

// Export for use in other modules
window.WebAuthnClient = WebAuthnClient; 