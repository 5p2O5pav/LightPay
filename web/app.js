// app.js - 支付页面逻辑
const orderId = window.location.pathname.split('/').pop();

// 获取订单信息
async function loadOrder() {
    try {
        const res = await fetch(`/api/order/${orderId}`);
        const data = await res.json();
        
        if (data.status === 'paid') {
            window.location.href = data.redirect_url;
            return;
        }
        
        document.getElementById('amount').textContent = `${data.amount} USDT`;
        document.getElementById('fiat-amount').textContent = `≈ ¥${data.fiat_amount}`;
        document.getElementById('address').textContent = data.address;
        
        // 生成二维码
        const qrcode = new QRCode(document.getElementById('qrcode'), {
            text: `tron:${data.address}?amount=${data.amount}`,
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

// 倒计时
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

// 轮询支付状态
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
                    window.location.href = data.redirect_url;
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

// 复制地址
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
