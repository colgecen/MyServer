import HeaderBar from "./components/HeaderBar";
import LeftSidebar from "./components/LeftSidebar";
import MainCanvas from "./components/MainCanvas";
import InputHUD from "./components/InputHUD";
export default function App() {
  return (
    <div className="app-shell" data-testid="app-shell">
      <HeaderBar />
      <div className="app-body">
        <LeftSidebar />
        <MainCanvas />
      </div>
      <InputHUD />
    </div>
  );
}
