import { Link } from "@tanstack/solid-router";
import { For, type Component } from "solid-js";
import type { Icon } from "phosphor-solid";
import type { AccountTab } from "~/routes/account";

// Define a generic type for the tab items that the parent will provide
export interface TabItem<T extends string> {
  id: T;
  label: string;
  icon: Icon;
}

interface AccountSidebarProps {
  user: {
    nickname: string;
    avatarUrl: string;
  };
  tabs: TabItem<AccountTab>[];
  activeTab: AccountTab;
}

export const AccountSidebar: Component<AccountSidebarProps> = (props) => {
  return (
    <aside class="md:w-64 flex-shrink-0">
      <div class="flex flex-col items-center p-4 border rounded-lg bg-white">
        <img
          src={props.user.avatarUrl}
          alt="User Avatar"
          class="w-24 h-24 rounded-full"
        />
        <h2 class="mt-3 text-xl font-semibold">{props.user.nickname}</h2>
      </div>
      <nav class="mt-6">
        <ul class="space-y-1">
          <For each={props.tabs}>
            {(tab) => {
              const Icon = tab.icon;
              const isActive = () => props.activeTab === tab.id;
              return (
                <li>
                  <Link
                    to="/account"
                    search={{ tab: tab.id }}
                    class="flex items-center gap-3 px-4 py-2.5 rounded-md text-gray-700 transition-colors"
                    classList={{
                      "bg-blue-100 text-blue-700 font-semibold": isActive(),
                      "hover:bg-gray-100": !isActive(),
                    }}
                  >
                    <Icon size={20} />
                    <span>{tab.label}</span>
                  </Link>
                </li>
              );
            }}
          </For>
        </ul>
      </nav>
    </aside>
  );
};
