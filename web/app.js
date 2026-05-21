// app.js - 支付页面逻辑（多链动态适配）
const orderId = window.location.pathname.split('/').pop();

// 获取订单信息
async function loadOrder() {
    try {
        const res = await fetch(`/api/order/${orderId}`);
        const data = await res.json();
        
        // 已支付则跳转
        if (data.status === 'paid') {
            window.location.href = data.redirect_url || '/';
            return;
        }
        
        // 更新页面文字
        document.getElementById('token-title').textContent = `${data.token.toUpperCase()} 支付`;
        document.getElementById('amount').textContent = `${data.amount} ${data.token.toUpperCase()}`;
        // 法币金额（如果后端未提供 fiat_amount 则显示占位）
        const fiatEl = document.getElementById('fiat-amount');
        if (data.fiat_amount) {
            fiatEl.textContent = `≈ ¥${data.fiat_amount}`;
        } else {
            fiatEl.textContent = '';
        }
        document.getElementById('address').textContent = data.address;
        document.getElementById('network-tag').textContent = data.network.toUpperCase();
        
        // 生成二维码（根据链类型选择协议）
        const qrText = generateQRText(data.network, data.address, data.amount);
        const qrContainer = document.getElementById('qrcode');
        qrContainer.innerHTML = ''; // 清除旧二维码
        new QRCode(qrContainer, {
            text: qrText,
            width: 160,
            height: 160,
        });
        
        // 启动倒计时
        startCountdown(data.expired_at);
        
        // 轮询支付状态
        pollPaymentStatus();
    } catch (err) {
        console.error('加载订单失败:', err);
    }
}

// 根据网络类型生成二维码内容
function generateQRText(network, address, amount) {
    switch (network.toLowerCase()) {
        case 'tron':
            return `tron:${address}?amount=${amount}`;
        case 'polygon':
        case 'bsc':
        case 'ethereum':
            // 以太坊系钱包通常支持 ethereum: 协议
            return `ethereum:${address}?value=${amount}`;
        default:
            // 默认仅显示地址
            return address;
    }
}

// 倒计时（保持不变，只需注意过期文字颜色）
function startCountdown(expireTime) {
    const timer = document.getElementById('timer');
    const updateTimer = () => {
        const now = Math.floor(Date.now() / 1000);
        const remaining = expireTime - now;
        
        if (remaining <= 0) {
            timer.textContent = '订单已过期';
            timer.parentElement.style.background = '#fee2e2';
            timer.parentElement.style.color = '#dc2626';
            return;
        }
        
        const minutes = Math.floor(remaining / 60);
        const seconds = remaining % 60;
        timer.textContent = `剩余 ${minutes}:${seconds.toString().padStart(2, '0')}`;
    };
    
    updateTimer();
    setInterval(updateTimer, 1000);
}

// 轮询支付状态（支付成功后跳转）
async function pollPaymentStatus() {
    const checkStatus = async () => {
        try {
            const res = await fetch(`/api/order/${orderId}/status`);
            const data = await res.json();
            
            if (data.status === 'paid') {
                document.getElementById('status').innerHTML = `
                    <div style="color: var(--success); font-size: 18px;">
                        ✅ 支付成功！正在跳转...
                    </div>
                `;
                setTimeout(() => {
                    window.location.href = data.redirect_url || '/';
                }, 2000);
                return;
            }
        } catch (err) {
            console.error('查询状态失败:', err);
        }
        
        setTimeout(checkStatus, 3000);
    };
    
    checkStatus();
}

// 复制地址（保持不变）
function copyAddress() {
    const address = document.getElementById('address').textContent;
    navigator.clipboard.writeText(address).then(() => {
        const btn = document.querySelector('.copy-btn');
        btn.textContent = '已复制!';
        btn.style.background = '#10b981';
        setTimeout(() => {
            btn.textContent = '复制';
            btn.style.background = '';
        }, 2000);
    });
}

// 页面加载
loadOrder();
