# Dashboard: giao diện gọn, năm ngôn ngữ và chuyển động mượt

## Mục tiêu đã thống nhất

AI Gateway là công cụ theo dõi usage và memory, không phải landing page.
Người dùng muốn giao diện đơn giản, hiệu quả, dễ đọc, bớt cảm giác template AI;
toàn bộ frontend hỗ trợ EN, VI, KR, ZH, DE; animation được chăm chút.
Hướng giao diện trung tính và tiếng Trung giản thể đã được duyệt trong chat.
Tài liệu này chốt hành vi và tiêu chí nghiệm thu trước khi lập kế hoạch triển khai.

## 1. Thiết kế giao diện

- Nền trắng/xám trung tính, chữ đậm vừa đủ, một màu nhấn xanh; màu trạng thái
  chỉ dành cho thông tin trạng thái. Bỏ gradient nền và icon trang trí dư thừa.
- Tiêu đề là tên chức năng: Overview, Sessions, Insights, Local Memory,
  Data & Pricing; bỏ slogan và nhãn viết hoa lặp lại tên trang.
- Thang chữ dùng cho công cụ đọc dữ liệu: nội dung 14–16px, tiêu đề 22–28px;
  không giảm mọi thứ thành chữ nhỏ để nhét thêm dữ liệu.
- Giảm card lồng nhau, bóng và bo góc lớn; dùng đường phân cách, bảng và khoảng
  cách nhất quán. Giữ đơn vị, khoảng thời gian và ý nghĩa số liệu ở gần giá trị.
- Header gọn với điều hướng, bộ chọn ngôn ngữ và currency khi liên quan.
  Menu mobile phải tìm được mọi trang, không gây tràn ngang toàn trang.
- Giữ tính năng hiện có, deep link và trạng thái filter; không đổi API, cách tính
  usage, giá, currency được chọn hay phạm vi thời gian khi đổi giao diện/ngôn ngữ.

### Local Memory

Đầu trang là tên chức năng, trạng thái ngắn, thời điểm kiểm tra và nút refresh.
Readiness của LLM, embedder, Neo4j trình bày thành một nhóm nhỏ. Tiếp theo là
delivery theo ba nguồn và extraction queue: dùng bảng hoặc hàng số liệu rõ ràng,
không bọc từng con số trong nhiều tầng card. Trên mobile, các hàng tự xếp lại
nhưng vẫn giữ nhãn cạnh giá trị.

Cảnh báo cần xử lý luôn hiện. Giải thích dài, lỗi kỹ thuật chi tiết và thống kê
legacy/excluded nằm trong phần mở rộng. Không che số job lỗi khi một nguồn cũng
đang lỗi. Giữ phân biệt tin nhắn được nhận với job được xử lý; không dựng phần
trăm tiến độ từ hai tập dữ liệu khác nhau. Dữ liệu cũ phải có nhãn, dữ liệu chưa
biết không biến thành 0; readiness không được gọi là worker heartbeat.

### Các trang khác

Áp dụng chung typography, spacing, header và cách viết nhãn cho Overview,
Sessions, Insights, Data & Pricing, proxy diagnostics và màn hình nhập token.
Giữ chart và network explorer hiện có, nhưng dịch controls/tooltip/empty state
và giảm trang trí quanh chúng. Không mở thêm dự án thay chart engine.

## 2. Animation

Chuyển động tạo cảm giác liền mạch và phản hồi rõ; không làm chậm thao tác.
Các thông số sau là mục tiêu thiết kế, cần kiểm chứng trên trình duyệt:

- Đổi trang: fade + dịch nhẹ 4–6px, khoảng 180–240ms; nội dung sẵn dùng ngay,
  không phải đợi animation kết thúc. Không remount form/filter chỉ để chạy hiệu ứng.
- Điều hướng: indicator chuyển vị trí mượt khoảng 180–220ms, xử lý đúng khi
  label đổi ngôn ngữ hoặc menu xuống dòng. Không chỉ dùng màu để chỉ trang đang chọn.
- Nút và input: phản hồi hover/focus/press khoảng 100–150ms; thao tác bàn phím
  có focus ring rõ. Không dùng hiệu ứng nam châm hoặc con trỏ tùy biến.
- Section mới xuất hiện: stagger nhẹ, tổng thời gian dưới 300ms; không animate
  từng hàng trong bảng dài và không chạy lại sau mỗi lần polling 30 giây.
- Disclosure/dialog/menu: mở/đóng ngắn, không nhảy bố cục bất ngờ. Trạng thái lỗi
  không biến mất hoặc bị trì hoãn bởi exit animation.
- Số liệu và giá tiền hiển thị giá trị thật ngay, không chạy counter giả.
  Không shimmer lặp vô hạn trên dữ liệu đã tải, không confetti, không parallax.
- Ưu tiên transform/opacity, tận dụng CSS và primitive hiện có. Chỉ thêm thư viện
  motion nếu prototype chứng minh cần thiết; animation không thành phụ thuộc của logic.
- `prefers-reduced-motion: reduce`: bỏ slide/stagger/scale và chuyển động liên tục
  không cần thiết; vẫn giữ trạng thái loading bằng chữ và mọi phản hồi chức năng.
  Network explorer cũng cần được rà soát chuyển động tự động theo lựa chọn này.

## 3. i18n toàn frontend

Dùng i18next + react-i18next, không tự xây thư viện dịch. Resources nằm trong repo,
không gửi text giao diện, dữ liệu phiên hay lỗi tới dịch vụ dịch bên ngoài.

Ngôn ngữ hiển thị: English (`en`), Tiếng Việt (`vi`), 한국어 (`ko`),
简体中文 (`zh-Hans`), Deutsch (`de`). KR là yêu cầu tiếng Hàn; mã ngôn ngữ là `ko`.

- Bộ chọn dùng tên ngôn ngữ bản địa, không dùng cờ quốc gia. Đổi ngay không reload;
  lưu lựa chọn cục bộ. Lần đầu theo ngôn ngữ trình duyệt trong tập hỗ trợ, fallback EN.
  Với Chinese truyền thống không có bản dịch tương ứng, fallback EN; không gán nhãn
  giản thể thành phồn thể. `zh` và `zh-CN` được ánh xạ tới `zh-Hans`.
- Dịch toàn bộ nhãn, button, placeholder, tooltip, chart legend, empty/loading/error
  state, document title, accessible name và văn bản do frontend sinh ra.
- Tách resource theo nhóm chức năng; key có kiểu và kiểm tra tương thích resources.
  Dùng interpolation/plural theo ngôn ngữ, không ghép các mảnh câu tiếng Anh.
- Helpers định dạng dùng locale đang chọn với Intl cho số, số rút gọn, phần trăm,
  ngày/giờ và tiền. Locale không quyết định currency hay thay đổi ranh giới ngày của
  dữ liệu. Giữ timezone hiện hành và ghi rõ nơi có ý nghĩa nghiệp vụ.
- Logic health trả trạng thái/key và tham số, không đóng băng câu đã dịch trong cache.
  Đổi ngôn ngữ phải cập nhật cả trạng thái và lỗi đang hiển thị mà không cần fetch lại.
- Lỗi do frontend biết được dịch qua mã/status. Lỗi server chưa biết có thông báo
  hướng dẫn đã dịch và phần chi tiết nguyên văn; không đoán nghĩa bằng dịch tự động.
- Không dịch model ID, tên sản phẩm, session title do người dùng tạo, workspace,
  đường dẫn, query người dùng, prompt hay response. Cập nhật thuộc tính `html lang`.
- Lưu preference thất bại không làm app crash; missing key fallback EN nhưng test
  phải bắt resource thiếu, tránh coi fallback là bản dịch hoàn chỉnh.

## 4. Kiểm thử và nghiệm thu

- Test locale detection, persistence/fallback, resource key/interpolation/plural,
  đổi locale khi dữ liệu đã cache, định dạng số/ngày/tiền và các trạng thái lỗi.
- Rà soát string hard-code: JSX, aria/title/placeholder, tooltip/chart/canvas text,
  document title, helper/hook và errors; loại trừ tên riêng, dữ liệu và protocol keys.
- Chạy toàn bộ frontend tests, typecheck/build và lint; giữ các test nghiệp vụ hiện có.
- Kiểm tra browser đủ năm ngôn ngữ, desktop/tablet/mobile; ưu tiên nhãn tiếng Đức
  dài và font Hàn/Trung. Không tràn ngang toàn trang tại 320/390/640/768px.
- Kiểm tra keyboard, focus sau điều hướng, mở disclosure, đổi ngôn ngữ, refresh,
  loading/error/disabled/stale và reduced motion. Không mất filter do animation.
- Kiểm tra chuyển trang liên tiếp và polling: không giật, không reanimate toàn màn
  hình, không nhân đôi request hoặc timer; không animate mọi hàng của bảng lớn.
- Build và cập nhật riêng gateway local, xác nhận dashboard thật ở :8788; không
  restart Graphiti hoặc worker chỉ để thay UI. Báo riêng lỗi dữ liệu backend nếu có.
- Bản dịch phải đầy đủ và nhất quán thuật ngữ; không tuyên bố đã được người bản ngữ
  kiểm duyệt nếu chưa có người thực hiện.

## Ngoài phạm vi

Không đổi kiến trúc microservices, contract ingest, Graphiti extraction, database,
auth/permissions, currency conversion, ngôn ngữ dữ liệu người dùng, hoặc deploy cloud.
Giữ nguyên mọi thay đổi backend và tài liệu kiến trúc đang có trong working tree.

## Cơ sở nghiên cứu

- NN/g: giảm thông tin và trang trí cạnh tranh với tác vụ chính, không xóa thông tin
  cần thiết chỉ để trông tối giản: https://www.nngroup.com/articles/aesthetic-minimalist-design/
- Carbon: dữ liệu so sánh dùng bảng, phần ít dùng mở rộng theo nhu cầu:
  https://carbondesignsystem.com/components/data-table/usage/
- react-i18next: https://react.i18next.com/getting-started
- i18next formatting và plural:
  https://www.i18next.com/translation-function/formatting
  https://www.i18next.com/translation-function/plurals

## Trạng thái

Hướng thiết kế trong chat đã được người dùng duyệt, bao gồm yêu cầu animation.
Bản spec này đang chờ duyệt trước bước lập kế hoạch và triển khai.
