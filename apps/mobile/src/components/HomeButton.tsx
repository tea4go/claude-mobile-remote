import home from '../assets/icons/svgs/home.svg'

export default function HomeButton({ onClick }: { onClick: () => void }) {
  return <button type="button" className="mobile-icon-button" aria-label="返回首页" onClick={onClick}>
    <img src={home} alt="" width={20} height={20} />
  </button>
}
