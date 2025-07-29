let ws;
let username;
let myKeyPair = null;
let otherClients = new Map(); // clientID -> publicKey
let isKeyGenerated = false;
let hasSharedKey = false; // Prevent infinite loop
let lastJoinedUser = null; // Track last user who joined
let needToShareBack = false; // Flag to share back when receiving a new key
let justSharedKey = false; // Track if we just shared our key recently

// Generate cryptographic keys on client side
async function generateKeyPair() {
    try {
        myKeyPair = await crypto.subtle.generateKey(
            {
                name: "RSA-OAEP",
                modulusLength: 2048,
                publicExponent: new Uint8Array([1, 0, 1]),
                hash: "SHA-256"
            },
            true,
            ["encrypt", "decrypt"]
        );
        isKeyGenerated = true;
        return true;
    } catch (error) {
        console.error('Failed to generate key pair:', error);
        return false;
    }
}

// Export public key for sharing
async function exportPublicKey() {
    try {
        const exported = await crypto.subtle.exportKey("spki", myKeyPair.publicKey);
        const result = btoa(String.fromCharCode(...new Uint8Array(exported)));
        return result;
    } catch (error) {
        console.error('Failed to export public key:', error);
        return null;
    }
}

// Encrypt message for a specific recipient
async function encryptMessage(message, recipientPublicKey) {
    try {
        // Import recipient's public key
        const keyData = Uint8Array.from(atob(recipientPublicKey), c => c.charCodeAt(0));
        const publicKey = await crypto.subtle.importKey(
            "spki",
            keyData,
            {
                name: "RSA-OAEP",
                hash: "SHA-256"
            },
            false,
            ["encrypt"]
        );
        
        // Encrypt the message
        const messageBytes = new TextEncoder().encode(message);
        const encrypted = await crypto.subtle.encrypt(
            {
                name: "RSA-OAEP"
            },
            publicKey,
            messageBytes
        );
        
        return btoa(String.fromCharCode(...new Uint8Array(encrypted)));
    } catch (error) {
        console.error('Failed to encrypt message:', error);
        return null;
    }
}

// Decrypt message with our private key
async function decryptMessage(encryptedMessage) {
    try {
        const encryptedBytes = Uint8Array.from(atob(encryptedMessage), c => c.charCodeAt(0));
        
        const decrypted = await crypto.subtle.decrypt(
            {
                name: "RSA-OAEP"
            },
            myKeyPair.privateKey,
            encryptedBytes
        );
        
        return new TextDecoder().decode(decrypted);
    } catch (error) {
        console.error('Failed to decrypt message:', error);
        return '[DECRYPTION FAILED]';
    }
}

async function sharePublicKey() {
    // Allow sharing if we haven't shared yet, OR if we need to share back after receiving a key
    // AND if we haven't just shared our key recently
    const canShare = (!hasSharedKey || needToShareBack) && !justSharedKey;
    
    if (ws && ws.readyState === WebSocket.OPEN && isKeyGenerated && canShare) {
        const publicKey = await exportPublicKey();
        if (publicKey) {
            const keyShareMsg = {
                type: 'public_key_share',
                content: publicKey, // Use actual public key as content
                sender: username,
                timestamp: Math.floor(Date.now() / 1000) // Convert to seconds
            };
            ws.send(JSON.stringify(keyShareMsg));
            hasSharedKey = true; // Prevent resharing
            needToShareBack = false; // Reset the flag after sharing
            
            // Set flag to prevent immediate resharing
            justSharedKey = true;
            setTimeout(() => {
                justSharedKey = false;
            }, 500); // Reset flag after 500ms
            
            return true;
        } else {
            return false;
        }
    } else {
        return false;
    }
}

function updateClientsList() {
    const clientsList = document.getElementById('clientsList');
    clientsList.innerHTML = '';
    
    // Add current user first
    const currentUserItem = document.createElement('div');
    currentUserItem.className = 'client-item';
    currentUserItem.innerHTML = `
        <span class="client-username">${username} (you)</span>
        <span class="lock-icon" title="Your Public Key (Click to copy)" onclick="copyMyPublicKey()">🔒</span>
    `;
    clientsList.appendChild(currentUserItem);
    
    // Collect and sort other clients alphabetically
    const sortedClients = Array.from(otherClients.keys()).sort((a, b) => a.localeCompare(b));
    for (const clientID of sortedClients) {
        const publicKey = otherClients.get(clientID);
        if (clientID === username) continue; // skip self if present
        const clientItem = document.createElement('div');
        clientItem.className = 'client-item';
        clientItem.innerHTML = `
            <span class="client-username">${clientID}</span>
            <span class="lock-icon" title="${clientID}'s Public Key (Click to copy)" onclick="copyPublicKey('${clientID}', '${publicKey}')">🔒</span>
        `;
        clientsList.appendChild(clientItem);
    }
}

async function copyMyPublicKey() {
    try {
        const myPublicKey = await exportPublicKey();
        if (myPublicKey) {
            await navigator.clipboard.writeText(myPublicKey);
            showCopyFeedback();
        }
    } catch (err) {
        console.error('Failed to copy public key:', err);
    }
}

async function copyPublicKey(clientID, publicKey) {
    try {
        await navigator.clipboard.writeText(publicKey);
        showCopyFeedback();
    } catch (err) {
        console.error('Failed to copy public key:', err);
    }
}

function showCopyFeedback() {
    // Simple visual feedback - could be enhanced with a toast notification
    const event = new Event('copy');
    document.dispatchEvent(event);
}

async function displayMessage(message) {
    const messagesDiv = document.getElementById('messages');
    const messageDiv = document.createElement('div');
    
    let className = 'message other';
    if (message.sender === username) {
        className = 'message own';
    } else if (message.type === 'system') {
        className = 'message system';
    }
    
    // Format timestamp
    const timestamp = message.timestamp ? new Date(message.timestamp * 1000) : new Date();
    const timeString = timestamp.toLocaleTimeString('en-US', { 
        hour12: false, 
        hour: '2-digit', 
        minute: '2-digit', 
        second: '2-digit' 
    });
    
    let displayText = `[${timeString}] ${message.sender}: `;
    
    if (message.type === 'encrypted_message') {
        // Only try to decrypt messages that were encrypted for us
        if (message.recipient !== username) {
            return;
        }
        
        // Only try to decrypt messages from others (not from ourselves)
        if (message.sender !== username) {
            const decryptedContent = await decryptMessage(message.content);
            displayText += decryptedContent;
        } else {
            // Skip our own encrypted messages (they were meant for others)
            return;
        }
    } else if (message.type === 'public_key_share') {
        // Store the client's public key silently
        if (message.content && message.sender !== username) {
            // Check if we already have this client's key before storing
            const alreadyHaveKey = otherClients.has(message.sender);
            
            otherClients.set(message.sender, message.content);
            
            // Update clients list immediately when we receive a new public key
            updateClientsList();
            
            // If we just received a new client's public key and we didn't already have it, share ours back
            // AND if we haven't just shared our key recently
            if (isKeyGenerated && !alreadyHaveKey && !justSharedKey) {
                needToShareBack = true;
                setTimeout(() => sharePublicKey(), 100); // Small delay to avoid race condition
            }
        }
        // Don't display anything for public key sharing
        return;
    } else if (message.type === 'request_keys') {
        // Another client is requesting our public key
        if (message.sender !== username && isKeyGenerated) {
            // Share our public key with the requesting client
            setTimeout(() => sharePublicKey(), 100);
        }
        return; // Don't display this message
    } else if (message.type === 'system') {
        // Remove "User " prefix from join/leave system messages
        let content = message.content;
        if (content.match(/^User (.+) joined the chat/)) {
            content = content.replace(/^User /, '');
        } else if (content.match(/^User (.+) left the chat/)) {
            content = content.replace(/^User /, '');
        }
        displayText = `[${timeString}] ${content}`; // Add timestamp to system messages
        
        // Handle user join/leave events
        if (message.content.includes('joined the chat') && isKeyGenerated) {
            // Extract username from "User X joined the chat"
            const match = message.content.match(/User (.+) joined the chat/);
            if (match) {
                lastJoinedUser = match[1];
            }
        } else if (message.content.includes('left the chat')) {
            // Extract username from "User X left the chat" and remove from clients list
            const match = message.content.match(/User (.+) left the chat/);
            if (match) {
                const leftUsername = match[1];
                // Remove the user from otherClients map
                otherClients.delete(leftUsername);
                // Update the clients list to reflect the change
                updateClientsList();
            }
        }
    } else {
        displayText += message.content;
    }
    
    messageDiv.className = className;
    messageDiv.textContent = displayText;
    
    messagesDiv.appendChild(messageDiv);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

async function sendMessage() {
    const messageInput = document.getElementById('messageInput');
    const message = messageInput.value.trim();
    
    if (message && ws && ws.readyState === WebSocket.OPEN) {
        // Display our own message locally
        const localMessage = {
            type: 'message',
            content: message,
            sender: username,
            timestamp: Math.floor(Date.now() / 1000)
        };
        displayMessage(localMessage);
        
        // Send encrypted message to all other clients (except ourselves)
        if (otherClients.size > 0) {
            for (const [clientID, publicKey] of otherClients) {
                // Skip sending to ourselves
                if (clientID === username) {
                    continue;
                }
                const encryptedContent = await encryptMessage(message, publicKey);
                if (encryptedContent) {
                    const encryptedMsg = {
                        type: 'encrypted_message',
                        content: encryptedContent,
                        sender: username,
                        recipient: clientID, // <-- NEW!
                        timestamp: Math.floor(Date.now() / 1000)
                    };
                    ws.send(JSON.stringify(encryptedMsg));
                }
            }
        }
        
        messageInput.value = '';
    }
}

function connect() {
    // Generate a username
    username = prompt("Enter your username:") || "Anonymous";
    if (username === "Anonymous") {
        username = username + "_" + Date.now();
    }
    
    // Generate keys first
    generateKeyPair().then(() => {
        // Connect to WebSocket
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws?username=${encodeURIComponent(username)}`;
        
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            document.getElementById('connectionStatus').textContent = 'Connected';
            document.getElementById('connectionStatus').className = 'connection-status status-connected';
            document.getElementById('messageInput').disabled = false;
            document.getElementById('sendButton').disabled = false;
            document.getElementById('messageInput').focus();
            
            // Update clients list to show current user
            updateClientsList();
            
            // Share public key after connection to trigger key exchange with existing clients
            sharePublicKey();
            
            // Also request existing clients to share their keys
            setTimeout(() => {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    const requestMsg = {
                        type: 'request_keys',
                        sender: username,
                        timestamp: Math.floor(Date.now() / 1000)
                    };
                    ws.send(JSON.stringify(requestMsg));
                }
            }, 500); // Small delay to ensure connection is stable
        };
        
        ws.onmessage = function(event) {
            const message = JSON.parse(event.data);
            displayMessage(message);
        };
        
        ws.onclose = function() {
            document.getElementById('connectionStatus').textContent = 'Disconnected';
            document.getElementById('connectionStatus').className = 'connection-status status-disconnected';
            document.getElementById('messageInput').disabled = true;
            document.getElementById('sendButton').disabled = true;
        };
        
        ws.onerror = function(error) {
            console.error('WebSocket error:', error);
            // Also disable input and button on error
            document.getElementById('messageInput').disabled = true;
            document.getElementById('sendButton').disabled = true;
        };
    });
}

// Event listeners
document.getElementById('sendButton').addEventListener('click', sendMessage);
document.getElementById('messageInput').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        sendMessage();
    }
});

// Initialize
connect(); 