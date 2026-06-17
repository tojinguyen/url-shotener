import http from 'k6/http';
import { check, sleep, fail } from 'k6';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

export const options = {
  stages: [
    { duration: '1m', target: 500 },  // Tăng tải lên 500 VUs
    { duration: '3m', target: 1000 }, // Chạy ổn định ở 1000 VUs
    { duration: '1m', target: 2000 }, // Đẩy lên đỉnh 2000 VUs
    { duration: '1m', target: 0 },   // Giảm tải về 0
  ],
  thresholds: {
    // Chỉ cảnh báo nếu tỷ lệ lỗi thực sự (5xx hoặc lỗi kết nối) vượt quá 1%
    http_req_failed: ['rate<0.01'],
    'http_req_duration': ['p(95)<100'],
  },
};

const WRITE_URL = 'http://localhost:8080/api/v1/shorten';
const READ_BASE = 'http://localhost:8081';

// --- Cấu hình kịch bản (có thể override qua biến môi trường __ENV) ---
// Kích thước pool key thật được tạo trong setup().
const POOL_SIZE = Number(__ENV.POOL_SIZE) || 1000;
// Tỷ lệ key "nóng" (hot) trong pool — phần này nhận phần lớn lưu lượng đọc
// để mô phỏng phân phối truy cập thực tế và đẩy cache hit ratio lên cao.
const HOT_FRACTION = Number(__ENV.HOT_FRACTION) || 0.10;

// Tỷ lệ WRITE / READ trên tổng lưu lượng.
const WRITE_RATIO = Number(__ENV.WRITE_RATIO) || 0.10;

// Phân phối nội bộ của luồng READ (tổng = 1.0):
//   HOT  -> đọc key nóng       => gần như luôn cache hit
//   COLD -> đọc key còn lại     => cache hit sau lần chạm đầu (lazy repopulate)
//   MISS -> key ngẫu nhiên 7 ký tự => cache miss + 404 thật
const READ_HOT = Number(__ENV.READ_HOT) || 0.80;
const READ_COLD = Number(__ENV.READ_COLD) || 0.15;
// READ_MISS = phần còn lại (~0.05)

// setup() chạy MỘT LẦN trước khi load test bắt đầu. Giá trị trả về được k6
// truyền (read-only) vào hàm default của MỌI VU — đây là cách chuẩn để chia sẻ
// trạng thái bất biến giữa các VU (VU không chia sẻ được biến runtime với nhau).
export function setup() {
  const keys = [];
  const params = {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: 'Setup_Seed' },
  };

  for (let i = 0; i < POOL_SIZE; i++) {
    const payload = JSON.stringify({
      long_url: `https://example.com/seed/${i}/${randomString(10)}`,
    });
    const res = http.post(WRITE_URL, payload, params);
    if (res.status === 201) {
      try {
        const key = JSON.parse(res.body).short_url;
        if (key) {
          keys.push(key);
        }
      } catch (e) {
        // Bỏ qua response không parse được; sẽ kiểm tra tổng số ở dưới.
      }
    }
  }

  if (keys.length === 0) {
    fail(
      `setup() không tạo được key nào (POOL_SIZE=${POOL_SIZE}). ` +
      `Kiểm tra service WRITE tại ${WRITE_URL} đã chạy chưa.`
    );
  }

  console.log(`setup(): đã tạo pool gồm ${keys.length}/${POOL_SIZE} key thật.`);
  return { keys };
}

export default function (data) {
  const keys = data.keys;
  // Ranh giới giữa tập key nóng (đầu mảng) và phần còn lại.
  const hotCount = Math.max(1, Math.floor(keys.length * HOT_FRACTION));

  if (Math.random() < WRITE_RATIO) {
    // --- LUỒNG WRITE ---
    const payload = JSON.stringify({
      long_url: `https://example.com/path/${randomString(10)}`,
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
      },
      // Sử dụng thẻ 'tags' với thuộc tính 'name' để k6 gộp nhóm toàn bộ request WRITE
      // tránh sinh ra hàng nghìn metric riêng lẻ gây cảnh báo tốn bộ nhớ RAM.
      tags: { name: 'Write_Shorten' },
    };

    const res = http.post(WRITE_URL, payload, params);
    check(res, {
      'write status is 201': (r) => r.status === 201,
      'has short_url': (r) => {
        try {
          return JSON.parse(r.body).short_url !== undefined;
        } catch (e) {
          return false;
        }
      },
    });
  } else {
    // --- LUỒNG READ (phân phối hot/cold/miss) ---
    const roll = Math.random();
    let shortKey;

    if (roll < READ_HOT) {
      // Key nóng: chọn ngẫu nhiên trong tập hot => cache hit gần như chắc chắn.
      shortKey = keys[Math.floor(Math.random() * hotCount)];
    } else if (roll < READ_HOT + READ_COLD) {
      // Key nguội: chọn trong phần còn lại của pool (key thật, đã tồn tại trong DB).
      const coldRange = keys.length - hotCount;
      const idx = coldRange > 0
        ? hotCount + Math.floor(Math.random() * coldRange)
        : Math.floor(Math.random() * keys.length);
      shortKey = keys[idx];
    } else {
      // Cache miss thật: key ngẫu nhiên 7 ký tự gần như chắc chắn không tồn tại.
      shortKey = randomString(7);
    }

    const params = {
      redirects: 0,
      tags: { name: 'Read_Redirect' }, // Gộp toàn bộ request READ vào một nhóm metric chung
    };

    const res = http.get(`${READ_BASE}/${shortKey}`, params);

    // Ghi đè chỉ định nghĩa lỗi của k6 cho request này:
    // Chỉ coi là thất bại nếu status từ 500 trở lên hoặc bằng 0 (lỗi mạng)
    res.status >= 500 || res.status === 0 ? res.error = 'Server Error' : null;

    check(res, {
      'read status is 302 or 404': (r) => r.status === 302 || r.status === 404,
    });
  }

  sleep(0.1);
}
