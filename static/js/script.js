// Message type constants (matching server constants)
const MESSAGE_TYPES = {
    SYSTEM: 'system',
    ENCRYPTED: 'encrypted_message',
    PUBLIC_KEY_SHARE: 'public_key_share',
    REQUEST_KEYS: 'request_keys',
    USER_INFO: 'user_info',
    LOCAL: 'local_message' // For local display only
};

let ws;
let username;
let myKeyPair = null;

// Update the page title to show current user
function updateTitle() {
    if (username) {
        document.title = `Chapp - ${username}`;
    } else {
        document.title = 'Chapp - E2E Chat';
    }
}
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
        
        // RSA-2048 can encrypt up to ~190 bytes, so we need to chunk longer messages
        const maxChunkSize = 180; // Conservative size to account for padding
        
        if (message.length <= maxChunkSize) {
            // Message is short enough to encrypt directly
            const messageBytes = new TextEncoder().encode(message);
            const encrypted = await crypto.subtle.encrypt(
                {
                    name: "RSA-OAEP"
                },
                publicKey,
                messageBytes
            );
            
            return btoa(String.fromCharCode(...new Uint8Array(encrypted)));
        }
        
        // Message is too long, split into chunks
        const chunks = splitMessageIntoChunks(message, maxChunkSize);
        const encryptedChunks = [];
        
        for (let i = 0; i < chunks.length; i++) {
            const chunk = chunks[i];
            const messageBytes = new TextEncoder().encode(chunk);
            const encrypted = await crypto.subtle.encrypt(
                {
                    name: "RSA-OAEP"
                },
                publicKey,
                messageBytes
            );
            
            encryptedChunks.push(btoa(String.fromCharCode(...new Uint8Array(encrypted))));
        }
        
        // Join encrypted chunks with a separator
        return encryptedChunks.join('|');
    } catch (error) {
        console.error('Failed to encrypt message:', error);
        return null;
    }
}

// Helper function to split message into chunks
function splitMessageIntoChunks(message, chunkSize) {
    const chunks = [];
    for (let i = 0; i < message.length; i += chunkSize) {
        const end = Math.min(i + chunkSize, message.length);
        chunks.push(message.substring(i, end));
    }
    return chunks;
}

// Decrypt message with our private key
async function decryptMessage(encryptedMessage) {
    try {
        // Check if this is a chunked message (contains separator)
        if (encryptedMessage.includes('|')) {
            // Handle chunked message
            const chunks = encryptedMessage.split('|');
            const decryptedChunks = [];
            
            for (let i = 0; i < chunks.length; i++) {
                const chunk = chunks[i];
                const encryptedBytes = Uint8Array.from(atob(chunk), c => c.charCodeAt(0));
                
                const decrypted = await crypto.subtle.decrypt(
                    {
                        name: "RSA-OAEP"
                    },
                    myKeyPair.privateKey,
                    encryptedBytes
                );
                
                decryptedChunks.push(new TextDecoder().decode(decrypted));
            }
            
            return decryptedChunks.join('');
        }
        
        // Handle single chunk message (original behavior)
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
                type: MESSAGE_TYPES.PUBLIC_KEY_SHARE,
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
    } else if (message.type === MESSAGE_TYPES.SYSTEM) {
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
    
    let displayText = '';
    let messageContent = '';
    
    if (message.type === MESSAGE_TYPES.ENCRYPTED) {
        // Only try to decrypt messages that were encrypted for us
        if (message.recipient !== username) {
            return;
        }
        
        // Only try to decrypt messages from others (not from ourselves)
        if (message.sender !== username) {
            const decryptedContent = await decryptMessage(message.content);
            messageContent = decryptedContent;
        } else {
            // Skip our own encrypted messages (they were meant for others)
            return;
        }

    } else if (message.type === MESSAGE_TYPES.PUBLIC_KEY_SHARE) {
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
    } else if (message.type === MESSAGE_TYPES.LOCAL) {
        // Display local messages (our own messages for local display)
        messageContent = message.content;
    } else if (message.type === MESSAGE_TYPES.REQUEST_KEYS) {
        // Another client is requesting our public key
        if (message.sender !== username && isKeyGenerated) {
            // Share our public key with the requesting client
            setTimeout(() => sharePublicKey(), 100);
        }
        return; // Don't display this message
    } else if (message.type === MESSAGE_TYPES.SYSTEM) {
        // Remove "User " prefix from join/leave system messages
        let content = message.content;
        if (content.match(/^User (.+) joined the chat/)) {
            content = content.replace(/^User /, '');
        } else if (content.match(/^User (.+) left the chat/)) {
            content = content.replace(/^User /, '');
        }
        messageContent = content;
        
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
        messageContent = message.content;
    }
    
    messageDiv.className = className;
    
    if (message.type === MESSAGE_TYPES.SYSTEM) {
        // System messages with simple structure
        messageDiv.innerHTML = `
            <div class="message-content">
                <span class="message-timestamp">${timeString}</span>
                <span class="message-text">${messageContent}</span>
            </div>
        `;
    } else {
        // Regular messages with structured content
        messageDiv.innerHTML = `
            <div class="message-header">
                <span class="message-username">${message.sender}</span>
                <span class="message-timestamp">${timeString}</span>
            </div>
            <div class="message-content">
                <span class="message-text">${messageContent}</span>
            </div>
        `;
    }
    
    messagesDiv.appendChild(messageDiv);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

async function sendMessage() {
    const messageInput = document.getElementById('messageInput');
    const message = messageInput.value.trim();
    
    if (message && ws && ws.readyState === WebSocket.OPEN && username !== "Loading...") {
        // Display our own message locally
        const localMessage = {
            type: MESSAGE_TYPES.LOCAL,
            content: message,
            sender: username,
            timestamp: Math.floor(Date.now() / 1000)
        };
        displayMessage(localMessage);
        
        // Send encrypted messages to all other clients (except ourselves)
        if (otherClients.size > 0) {
            for (const [clientID, publicKey] of otherClients) {
                // Skip sending to ourselves
                if (clientID === username) {
                    continue;
                }
                const encryptedContent = await encryptMessage(message, publicKey);
                if (encryptedContent) {
                    const encryptedMsg = {
                        type: MESSAGE_TYPES.ENCRYPTED,
                        content: encryptedContent,
                        sender: username,
                        recipient: clientID,
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
    // Username will be provided by the server via session
    // We'll get it from the WebSocket connection
    username = "Loading..."; // Will be updated when WebSocket connects
    
    // Update the title with the username
    updateTitle();
    
    // Generate keys first
    generateKeyPair().then(() => {
        // Connect to WebSocket (session will be sent via cookies)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            const connectionStatus = document.getElementById('connectionStatus');
            const connectionIcon = connectionStatus.querySelector('.connection-icon');
            connectionStatus.querySelector('.connection-text').textContent = 'Connected';
            connectionStatus.className = 'connection-indicator status-connected';
            connectionIcon.textContent = '🔗'; // Reset to link icon for connected state
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
                type: MESSAGE_TYPES.REQUEST_KEYS,
                sender: username,
                timestamp: Math.floor(Date.now() / 1000)
            };
                    ws.send(JSON.stringify(requestMsg));
                }
            }, 500); // Small delay to ensure connection is stable
        };
        
        ws.onmessage = function(event) {
            const message = JSON.parse(event.data);
            
            // Handle user_info message to get username from server
            if (message.type === MESSAGE_TYPES.USER_INFO) {
                username = message.content;
                updateTitle();
                updateClientsList(); // Update clients list with correct username
                return;
            }
            
            displayMessage(message);
        };
        
        ws.onclose = function() {
            const connectionStatus = document.getElementById('connectionStatus');
            const connectionIcon = connectionStatus.querySelector('.connection-icon');
            connectionStatus.querySelector('.connection-text').textContent = 'Disconnected';
            connectionStatus.className = 'connection-indicator status-disconnected';
            connectionIcon.textContent = '❌'; // Change to X icon for disconnected state
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

// Logout functionality
document.getElementById('logoutBtn').addEventListener('click', function() {
    // Close WebSocket connection
    if (ws) {
        ws.close();
    }
    
    // Redirect to server logout route
    window.location.href = '/logout';
});

// Initialize
connect(); 