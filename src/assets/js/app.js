// S3 Storage Web UI JavaScript

// Toast notification system
const Toast = {
    container: null,

    init() {
        this.container = document.createElement('div');
        this.container.className = 'toast-container';
        document.body.appendChild(this.container);
    },

    show(message, type = 'info', duration = 3000) {
        if (!this.container) this.init();

        // Ensure message is defined and not empty
        if (!message || message === 'undefined' || message === 'null') {
            console.error('Toast called with empty message:', message);
            message = 'An error occurred';
        }

        console.log('Toast message:', message, 'Type:', type);

        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;

        // Create span and button elements properly
        const messageSpan = document.createElement('span');
        messageSpan.textContent = message; // Use textContent to avoid XSS and ensure proper display

        const closeButton = document.createElement('button');
        closeButton.innerHTML = '&times;';
        closeButton.className = 'ml-2 hover:opacity-75';
        closeButton.onclick = function() { toast.remove(); };

        toast.appendChild(messageSpan);
        toast.appendChild(closeButton);

        this.container.appendChild(toast);

        setTimeout(() => toast.remove(), duration);
    },

    success(message) {
        console.log('Toast.success called with:', message);
        this.show(message, 'success');
    },
    error(message) {
        console.log('Toast.error called with:', message);
        this.show(message, 'error', 5000);
    },
    info(message) {
        console.log('Toast.info called with:', message);
        this.show(message, 'info');
    }
};

// Modal management
const Modal = {
    show(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.classList.remove('hidden');
            modal.classList.add('flex');
        }
    },

    hide(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
    },

    confirm(message, onConfirm) {
        const modal = document.getElementById('confirmModal');
        if (!modal) return;

        const msgEl = document.getElementById('confirmMessage');
        msgEl.textContent = message;
        // Expose full message on hover for long names
        msgEl.title = message;

        const btn = document.getElementById('confirmBtn');
        // Replace any previous handler to avoid duplicates
        if (btn) {
            btn.onclick = null;
            btn.onclick = () => {
                if (typeof onConfirm === 'function') {
                    try {
                        onConfirm();
                    } catch (e) {
                        console.error('Error in confirm handler', e);
                    }
                }
                this.hide('confirmModal');
            };
        }
        this.show('confirmModal');
    }
};

// Utility to safely encode object keys for inclusion in URLs while preserving folder separators
function encodeKeyForURL(key) {
    if (!key) return '';
    return key.split('/').map(part => encodeURIComponent(part)).join('/');
}

// File upload handling
const FileUpload = {
    init(dropZoneId, fileInputId, bucketName) {
        const dropZone = document.getElementById(dropZoneId);
        const fileInput = document.getElementById(fileInputId);

        if (!dropZone || !fileInput) return;

        // Click to select files
        dropZone.addEventListener('click', () => fileInput.click());

        // Drag and drop events
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('drag-over');
        });

        dropZone.addEventListener('dragleave', () => {
            dropZone.classList.remove('drag-over');
        });

        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('drag-over');

            const files = e.dataTransfer.files;
            if (files.length > 0) {
                this.uploadFiles(files, bucketName);
            }
        });

        // File input change
        fileInput.addEventListener('change', (e) => {
            if (e.target.files.length > 0) {
                this.uploadFiles(e.target.files, bucketName);
            }
        });
    },

    async uploadFiles(files, bucketName) {
        const progressContainer = document.getElementById('uploadProgress');
        const progressBar = document.getElementById('progressBar');
        const progressText = document.getElementById('progressText');
        const progressPercent = document.getElementById('progressPercent');
        const errorContainer = document.getElementById('uploadError');
        const errorMessage = document.getElementById('uploadErrorMessage');

        // Hide any previous errors
        if (errorContainer) {
            errorContainer.classList.add('hidden');
        }

        if (progressContainer) {
            progressContainer.classList.remove('hidden');
        }

        let uploaded = 0;
        const total = files.length;
        const errors = [];

        for (const file of files) {
            try {
                const key = this.getCurrentPath() + file.name;
                // Ensure key is safely encoded for use in URLs (preserve slashes)
                const encodedKey = encodeKeyForURL(key);
                await this.uploadFile(file, bucketName, encodedKey, (percent) => {
                    if (progressBar) {
                        progressBar.style.width = `${percent}%`;
                    }
                    if (progressPercent) {
                        progressPercent.textContent = `${percent}%`;
                    }
                    if (progressText) {
                        progressText.textContent = `Uploading ${file.name}...`;
                        // Set title so users can hover to see full file name
                        progressText.title = file.name;
                    }
                });
                uploaded++;
                Toast.success(`Uploaded ${file.name}`);
            } catch (error) {
                const errMsg = `${file.name}: ${error.message}`;
                errors.push(errMsg);
                Toast.error(`Failed to upload ${file.name}`);
            }
        }

        if (progressContainer) {
            progressContainer.classList.add('hidden');
        }

        // Show errors in the modal if any occurred
        if (errors.length > 0 && errorContainer && errorMessage) {
            errorMessage.innerHTML = errors.map(e => `<div>${e}</div>`).join('');
            errorContainer.classList.remove('hidden');
        }

        if (uploaded > 0) {
            // Reload the page to show new files
            window.location.reload();
        }
    },

    getCurrentPath() {
        const pathElement = document.getElementById('currentPath');
        if (pathElement) {
            const path = pathElement.dataset.path || '';
            return path ? path + '/' : '';
        }
        return '';
    },

    uploadFile(file, bucket, encodedKey, onProgress) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();

            // Set timeout for slow networks (10 minutes for large files)
            xhr.timeout = 600000;

            // Show initial progress when upload starts
            xhr.upload.addEventListener('loadstart', () => {
                onProgress(0);
            });

            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    const percent = Math.round((e.loaded / e.total) * 100);
                    onProgress(percent);
                }
            });

            xhr.addEventListener('load', () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    onProgress(100); // Ensure we show 100% on success
                    resolve(xhr.response);
                } else {
                    // Try to parse S3 XML error response
                    const errorMessage = this.parseS3Error(xhr.responseText, xhr.status);
                    reject(new Error(errorMessage));
                }
            });

            xhr.addEventListener('error', () => {
                // Try to get any response that might be available
                if (xhr.responseText) {
                    const errorMessage = this.parseS3Error(xhr.responseText, xhr.status || 0);
                    reject(new Error(errorMessage));
                } else {
                    reject(new Error('Network error - connection failed or server unavailable'));
                }
            });

            xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')));

            xhr.addEventListener('timeout', () => reject(new Error('Upload timed out - please try a smaller file or check your connection')));

            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            // Use encoded bucket and key when opening the request to avoid spaces/special chars breaking the HTTP request line
            xhr.open('PUT', `/${encodeURIComponent(bucket)}/${encodedKey}`);
            xhr.setRequestHeader('X-CSRF-Token', csrfToken);
            xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
            xhr.send(file);
        });
    },

    parseS3Error(responseText, status) {
        try {
            const parser = new DOMParser();
            const xml = parser.parseFromString(responseText, 'application/xml');

            // Try to get the Message element from S3 error response
            const messageEl = xml.getElementsByTagName('Message')[0];
            if (messageEl && messageEl.textContent) {
                return messageEl.textContent;
            }

            // Fall back to Code if Message not found
            const codeEl = xml.getElementsByTagName('Code')[0];
            if (codeEl && codeEl.textContent) {
                return this.getHumanReadableError(codeEl.textContent);
            }
        } catch (e) {
            // Parsing failed, fall through to default
        }

        return `Upload failed (HTTP ${status})`;
    },

    getHumanReadableError(code) {
        const errorMessages = {
            'QuotaExceeded': 'Storage quota exceeded. Please free up space or contact your administrator.',
            'AccessDenied': 'Access denied. You do not have permission to upload to this location.',
            'NoSuchBucket': 'The bucket does not exist.',
            'InvalidBucketName': 'Invalid bucket name.',
            'EntityTooLarge': 'The file is too large to upload (max 5GB per file).',
            'ObjectAlreadyExists': 'A file with this name already exists.',
            'InvalidObjectKey': 'Invalid file name.',
            'InternalError': 'Server error occurred. The file may be too large or the server is busy.'
        };
        return errorMessages[code] || `Error: ${code}`;
    }
};

// Share link management
const ShareLink = {
    async create(bucket, key) {
        const expiresIn = document.getElementById('shareExpiry')?.value || '86400';
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            // Encode key parts before sending
            const encodedKey = encodeKeyForURL(key);
            const response = await fetch(`/share/create/${encodeURIComponent(bucket)}/${encodedKey}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify({ expires_in: parseInt(expiresIn) })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            const data = await response.json();

            // Show the share URL
            const shareUrlInput = document.getElementById('shareUrl');
            if (shareUrlInput) {
                shareUrlInput.value = data.share_url;
            }

            Modal.show('shareResultModal');
            Toast.success('Share link created');
        } catch (error) {
            Toast.error(`Failed to create share link: ${error.message}`);
        }
    },

    async delete(token) {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            const response = await fetch(`/share/${token}`, {
                method: 'DELETE',
                headers: {
                    'X-CSRF-Token': csrfToken
                }
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            Toast.success('Share link deleted');
            window.location.reload();
        } catch (error) {
            Toast.error(`Failed to delete share link: ${error.message}`);
        }
    },

    copyToClipboard() {
        const shareUrlInput = document.getElementById('shareUrl');
        if (shareUrlInput) {
            shareUrlInput.select();
            document.execCommand('copy');
            Toast.success('Link copied to clipboard');
        }
    }
};

// Bucket management
const Bucket = {
    async create(name) {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            const response = await fetch(`/${name}`, {
                method: 'PUT',
                headers: {
                    'X-CSRF-Token': csrfToken
                }
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            Toast.success(`Bucket "${name}" created`);
            window.location.reload();
        } catch (error) {
            Toast.error(`Failed to create bucket: ${error.message}`);
        }
    },

    async delete(name) {
        Modal.confirm(`Are you sure you want to delete bucket "${name}"? This action cannot be undone.`, async () => {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            try {
                const response = await fetch(`/${name}`, {
                    method: 'DELETE',
                    headers: {
                        'X-CSRF-Token': csrfToken
                    }
                });

                if (response.status === 409) {
                    // Bucket not empty - ask user if they want to force delete
                    Modal.confirm(
                        `Bucket "${name}" is not empty. Do you want to delete all objects and then delete the bucket?`,
                        () => this.forceDelete(name)
                    );
                    return;
                }

                if (!response.ok) {
                    const text = await response.text();
                    throw new Error(text || `HTTP ${response.status}`);
                }

                Toast.success(`Bucket "${name}" deleted`);
                window.location.reload();
            } catch (error) {
                Toast.error(`Failed to delete bucket: ${error.message}`);
            }
        });
    },

    async forceDelete(name) {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            Toast.info(`Deleting all objects in "${name}"...`);

            // List all objects in the bucket
            const objects = await this.listAllObjects(name);

            if (objects.length > 0) {
                // Delete all objects
                let deleted = 0;
                for (const obj of objects) {
                    try {
                        // URL encode the key to handle spaces and special characters
                        const encodedKey = obj.key.split('/').map(part => encodeURIComponent(part)).join('/');
                        const deleteResponse = await fetch(`/${encodeURIComponent(name)}/${encodedKey}`, {
                            method: 'DELETE',
                            headers: {
                                'X-CSRF-Token': csrfToken
                            }
                        });

                        if (deleteResponse.ok || deleteResponse.status === 204) {
                            deleted++;
                        }
                    } catch (e) {
                        console.error(`Failed to delete object ${obj.key}:`, e);
                    }
                }
                Toast.info(`Deleted ${deleted} of ${objects.length} objects`);
            }

            // Now delete the bucket
            const response = await fetch(`/${name}`, {
                method: 'DELETE',
                headers: {
                    'X-CSRF-Token': csrfToken
                }
            });

            if (!response.ok) {
                const text = await response.text();
                throw new Error(text || `HTTP ${response.status}`);
            }

            Toast.success(`Bucket "${name}" and all its contents deleted`);
            window.location.reload();
        } catch (error) {
            Toast.error(`Failed to force delete bucket: ${error.message}`);
        }
    },

    async listAllObjects(bucketName) {
        const objects = [];

        try {
            // Fetch objects using the S3 ListObjectsV2 API (no delimiter to get all objects flat)
            const response = await fetch(`/${bucketName}?list-type=2&max-keys=10000`);

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            const text = await response.text();
            const parser = new DOMParser();
            const xml = parser.parseFromString(text, 'application/xml');

            // Extract object keys from XML response
            // Use getElementsByTagNameNS for namespace-aware parsing, or fall back to local name matching
            const ns = 'http://s3.amazonaws.com/doc/2006-03-01/';
            let contents = xml.getElementsByTagNameNS(ns, 'Contents');

            // Fallback for browsers that don't handle namespaces well
            if (contents.length === 0) {
                contents = xml.getElementsByTagName('Contents');
            }

            for (const content of contents) {
                let keyElement = content.getElementsByTagNameNS(ns, 'Key')[0];
                if (!keyElement) {
                    keyElement = content.getElementsByTagName('Key')[0];
                }
                if (keyElement) {
                    objects.push({ key: keyElement.textContent });
                }
            }
        } catch (error) {
            console.error('Failed to list objects:', error);
        }

        return objects;
    }
};

// Object management
const S3Object = {
    async delete(bucket, key) {
        Modal.confirm(`Are you sure you want to delete "${key}"?`, async () => {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            try {
                const encodedKey = encodeKeyForURL(key);
                const response = await fetch(`/${encodeURIComponent(bucket)}/${encodedKey}`, {
                    method: 'DELETE',
                    headers: {
                        'X-CSRF-Token': csrfToken
                    }
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }

                Toast.success(`"${key}" deleted`);
                window.location.reload();
            } catch (error) {
                Toast.error(`Failed to delete object: ${error.message}`);
            }
        });
    },

    download(bucket, key) {
        const encodedKey = encodeKeyForURL(key);
        window.location.href = `/${encodeURIComponent(bucket)}/${encodedKey}`;
    }
};

// Hash display and copy functionality
const HashDisplay = {
    copyHash(hash) {
        if (!hash) {
            Toast.error('No hash to copy');
            return;
        }

        // Try modern clipboard API first
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(hash)
                .then(() => {
                    Toast.success('Hash copied to clipboard');
                })
                .catch(err => {
                    console.error('Clipboard API failed:', err);
                    this.fallbackCopy(hash);
                });
        } else {
            // Fallback for older browsers
            this.fallbackCopy(hash);
        }
    },

    fallbackCopy(hash) {
        const textarea = document.createElement('textarea');
        textarea.value = hash;
        textarea.style.position = 'fixed';
        textarea.style.top = '0';
        textarea.style.left = '0';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();

        try {
            const successful = document.execCommand('copy');
            if (successful) {
                Toast.success('Hash copied to clipboard');
            } else {
                Toast.error('Failed to copy hash');
            }
        } catch (err) {
            console.error('Fallback copy failed:', err);
            Toast.error('Failed to copy hash');
        } finally {
            document.body.removeChild(textarea);
        }
    }
};

// Bulk actions for objects
const BulkActions = {
    getSelectedCheckboxes() {
        return document.querySelectorAll('.object-checkbox:checked');
    },

    getSelectedObjects() {
        const selected = [];
        this.getSelectedCheckboxes().forEach(checkbox => {
            selected.push({
                bucket: checkbox.dataset.bucket,
                key: checkbox.dataset.key
            });
        });
        return selected;
    },

    updateBulkActionsUI() {
        const selected = this.getSelectedCheckboxes();
        const bulkActionsBar = document.getElementById('bulkActionsBar');
        const selectedCount = document.getElementById('selectedCount');
        const selectAllCheckbox = document.getElementById('selectAll');

        if (selected.length > 0) {
            if (bulkActionsBar) {
                bulkActionsBar.classList.remove('hidden');
            }
            if (selectedCount) {
                selectedCount.textContent = `${selected.length} object${selected.length > 1 ? 's' : ''} selected`;
            }
        } else {
            if (bulkActionsBar) {
                bulkActionsBar.classList.add('hidden');
            }
            if (selectAllCheckbox) {
                selectAllCheckbox.checked = false;
            }
        }

        // Update select all checkbox state
        if (selectAllCheckbox) {
            const allCheckboxes = document.querySelectorAll('.object-checkbox');
            const checkedCheckboxes = document.querySelectorAll('.object-checkbox:checked');
            selectAllCheckbox.checked = allCheckboxes.length > 0 && allCheckboxes.length === checkedCheckboxes.length;
            selectAllCheckbox.indeterminate = checkedCheckboxes.length > 0 && checkedCheckboxes.length < allCheckboxes.length;
        }
    },

    toggleSelectAll(checkbox) {
        const objectCheckboxes = document.querySelectorAll('.object-checkbox');
        objectCheckboxes.forEach(cb => {
            cb.checked = checkbox.checked;
        });
        this.updateBulkActionsUI();
    },

    clearSelection() {
        const objectCheckboxes = document.querySelectorAll('.object-checkbox');
        objectCheckboxes.forEach(cb => {
            cb.checked = false;
        });
        const selectAllCheckbox = document.getElementById('selectAll');
        if (selectAllCheckbox) {
            selectAllCheckbox.checked = false;
        }
        this.updateBulkActionsUI();
    },

    async deleteSelected() {
        const objects = this.getSelectedObjects();

        if (objects.length === 0) {
            Toast.error('No objects selected');
            return;
        }

        const confirmMessage = objects.length === 1
            ? `Are you sure you want to delete "${objects[0].key}"?`
            : `Are you sure you want to delete ${objects.length} objects? This action cannot be undone.`;

        Modal.confirm(confirmMessage, async () => {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';
            let deleted = 0;
            let failed = 0;

            Toast.info(`Deleting ${objects.length} object${objects.length > 1 ? 's' : ''}...`);

            for (const obj of objects) {
                try {
                    const encodedKey = encodeKeyForURL(obj.key);
                    const response = await fetch(`/${encodeURIComponent(obj.bucket)}/${encodedKey}`, {
                        method: 'DELETE',
                        headers: {
                            'X-CSRF-Token': csrfToken
                        }
                    });

                    if (response.ok || response.status === 204) {
                        deleted++;
                    } else {
                        failed++;
                        console.error(`Failed to delete ${obj.key}: HTTP ${response.status}`);
                    }
                } catch (error) {
                    failed++;
                    console.error(`Failed to delete ${obj.key}:`, error);
                }
            }

            if (deleted > 0) {
                Toast.success(`Deleted ${deleted} object${deleted > 1 ? 's' : ''}`);
            }

            if (failed > 0) {
                Toast.error(`Failed to delete ${failed} object${failed > 1 ? 's' : ''}`);
            }

            // Reload the page to show updated list
            window.location.reload();
        });
    }
};

// User management
const User = {
    async create(formData) {
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            const response = await fetch('/ui/users', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify(formData)
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `HTTP ${response.status}`);
            }

            Toast.success('User created');
            window.location.reload();
        } catch (error) {
            Toast.error(`Failed to create user: ${error.message}`);
        }
    },

    async delete(userId) {
        Modal.confirm('Are you sure you want to delete this user?', async () => {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            try {
                const response = await fetch(`/ui/users/${userId}`, {
                    method: 'DELETE',
                    headers: {
                        'X-CSRF-Token': csrfToken
                    }
                });

                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || `HTTP ${response.status}`);
                }

                Toast.success('User deleted');
                window.location.reload();
            } catch (error) {
                Toast.error(`Failed to delete user: ${error.message}`);
            }
        });
    },

    async changePassword() {
        const currentPassword = document.getElementById('currentPassword').value;
        const newPassword = document.getElementById('newPasswordChange').value;
        const confirmPassword = document.getElementById('confirmNewPassword').value;

        if (newPassword !== confirmPassword) {
            Toast.error('Passwords do not match');
            return;
        }

        if (newPassword.length < 8) {
            Toast.error('Password must be at least 8 characters');
            return;
        }

        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            const response = await fetch('/ui/users/me/password', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify({
                    current_password: currentPassword,
                    new_password: newPassword
                })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `HTTP ${response.status}`);
            }

            Toast.success('Password changed successfully');
            Modal.hide('changePasswordModal');

            // Clear form
            document.getElementById('changePasswordForm').reset();
        } catch (error) {
            Toast.error(`Failed to change password: ${error.message}`);
        }
    },

    showResetPasswordModal(userId, username) {
        document.getElementById('resetPasswordUserId').value = userId;
        document.getElementById('resetPasswordUsername').textContent = username;
        document.getElementById('resetNewPassword').value = '';
        document.getElementById('resetConfirmPassword').value = '';
        Modal.show('resetPasswordModal');
    },

    async adminResetPassword() {
        const userId = document.getElementById('resetPasswordUserId').value;
        const newPassword = document.getElementById('resetNewPassword').value;
        const confirmPassword = document.getElementById('resetConfirmPassword').value;

        if (newPassword !== confirmPassword) {
            Toast.error('Passwords do not match');
            return;
        }

        if (newPassword.length < 8) {
            Toast.error('Password must be at least 8 characters');
            return;
        }

        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

        try {
            const response = await fetch(`/ui/users/${userId}/password`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify({
                    new_password: newPassword
                })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `HTTP ${response.status}`);
            }

            Toast.success('Password reset successfully');
            Modal.hide('resetPasswordModal');
        } catch (error) {
            Toast.error(`Failed to reset password: ${error.message}`);
        }
    }
};

// Stats and Charts
const Stats = {
    async load() {
        try {
            const response = await fetch('/api/stats');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            return await response.json();
        } catch (error) {
            console.error('Failed to load stats:', error);
            return null;
        }
    },

    renderStorageChart(canvasId, buckets) {
        const canvas = document.getElementById(canvasId);
        if (!canvas || !buckets || buckets.length === 0) return;

        const ctx = canvas.getContext('2d');
        const colors = [
            '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
            '#EC4899', '#06B6D4', '#84CC16', '#F97316', '#6366F1'
        ];

        new Chart(ctx, {
            type: 'pie',
            data: {
                labels: buckets.map(b => b.name),
                datasets: [{
                    data: buckets.map(b => b.total_size),
                    backgroundColor: colors.slice(0, buckets.length),
                    borderWidth: 1,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right'
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                const bytes = context.raw;
                                return `${context.label}: ${formatBytes(bytes)}`;
                            }
                        }
                    }
                }
            }
        });
    },

    renderObjectCountChart(canvasId, buckets) {
        const canvas = document.getElementById(canvasId);
        if (!canvas || !buckets || buckets.length === 0) return;

        const ctx = canvas.getContext('2d');
        const colors = [
            '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
            '#EC4899', '#06B6D4', '#84CC16', '#F97316', '#6366F1'
        ];

        new Chart(ctx, {
            type: 'pie',
            data: {
                labels: buckets.map(b => b.name),
                datasets: [{
                    data: buckets.map(b => b.object_count),
                    backgroundColor: colors.slice(0, buckets.length),
                    borderWidth: 1,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right'
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return `${context.label}: ${context.raw} objects`;
                            }
                        }
                    }
                }
            }
        });
    },

    renderContentTypeChart(canvasId, contentTypes) {
        const canvas = document.getElementById(canvasId);
        if (!canvas || !contentTypes || contentTypes.length === 0) return;

        const ctx = canvas.getContext('2d');
        const colors = [
            '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
            '#EC4899', '#06B6D4', '#84CC16', '#F97316', '#6366F1',
            '#14B8A6', '#A855F7', '#F43F5E', '#0EA5E9', '#22C55E'
        ];

        new Chart(ctx, {
            type: 'pie',
            data: {
                labels: contentTypes.map(ct => ct.label),
                datasets: [{
                    data: contentTypes.map(ct => ct.count),
                    backgroundColor: colors.slice(0, contentTypes.length),
                    borderWidth: 1,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right'
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                const ct = contentTypes[context.dataIndex];
                                return `${ct.label}: ${ct.count} files (${formatBytes(ct.total_size)})`;
                            }
                        }
                    }
                }
            }
        });
    }
};

// Utility functions
function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB'];

    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function formatDate(timestamp) {
    if (!timestamp) return '-';
    const date = new Date(timestamp * 1000);
    return date.toLocaleString();
}

function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        Toast.success('Copied to clipboard');
    }).catch(() => {
        Toast.error('Failed to copy');
    });
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    // Initialize toast container
    Toast.init();

    // Close modal on backdrop click
    document.querySelectorAll('.modal-backdrop').forEach(backdrop => {
        backdrop.addEventListener('click', (e) => {
            if (e.target === backdrop) {
                backdrop.classList.add('hidden');
                backdrop.classList.remove('flex');
            }
        });
    });

    // Close modal on Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-backdrop').forEach(modal => {
                modal.classList.add('hidden');
                modal.classList.remove('flex');
            });
        }
    });
});
