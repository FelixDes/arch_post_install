#include <yaml-cpp/yaml.h>
#include <ncurses.h>
#include <curl/curl.h>
#include <cstdlib>
#include <csignal>
#include <clocale>
#include <fstream>
#include <string>
#include <vector>
#include <memory>
#include <iostream>
#include <regex>
#include <chrono>
#include <cxxopts.hpp>

const std::string AUR_MANAGER = "yay --noconfirm --answerdiff=None --answeredit=None";
const std::string AUR_MANAGER_ALIAS = "__MGR__";
const std::regex AUR_MANAGER_ALIAS_REGEX = std::regex(AUR_MANAGER_ALIAS);

const std::string NOTIFY_COMMAND = "notify-send -i dialog-information -t 5000 -u critical";
const std::string NOTIFY_COMMAND_ALIAS = "__NOTIFY__";
const std::regex NOTIFY_COMMAND_ALIAS_REGEX = std::regex(NOTIFY_COMMAND_ALIAS);

// ----------- Actions -----------

struct Action {
    virtual ~Action() = default;

    virtual std::string render() const = 0;
};

struct YayAction : Action {
    std::string pkg;

    explicit YayAction(const std::string &p) : pkg(p) {
    }

    std::string render() const override {
        return AUR_MANAGER + " -S " + pkg;
    }
};

struct ShAction : Action {
    std::vector<std::string> commands;

    explicit ShAction(const std::vector<std::string> &cmds) : commands(cmds) {
    }

    std::string render() const override {
        std::string script;
        for (auto &cmd: commands) {
            script += cmd + " && ";
        }
        if (!script.empty()) script.erase(script.size() - 4);
        script = std::regex_replace(script, AUR_MANAGER_ALIAS_REGEX, AUR_MANAGER);
        script = std::regex_replace(script, NOTIFY_COMMAND_ALIAS_REGEX, NOTIFY_COMMAND);
        return script;
    }
};

// ----------- Menu -----------

enum ItemType { ITEM_CHECKBOX, ITEM_SECTION };

struct MenuItem {
    std::string label;
    ItemType type = ITEM_CHECKBOX;
    bool checked = true; // по умолчанию ВСЕ включены
    std::vector<MenuItem> children;
    std::shared_ptr<Action> action;
};

MenuItem parse_item(const YAML::Node &node) {
    MenuItem item;

    if (node.IsScalar()) {
        // просто строка → yay пакет
        item.label = node.as<std::string>();
        item.type = ITEM_CHECKBOX;
        item.checked = true;
        item.action = std::make_shared<YayAction>(item.label);
    } else if (node.IsMap()) {
        // объект с полями
        if (node["name"] && node["name"].IsScalar()) {
            item.label = node["name"].as<std::string>();
            item.type = ITEM_CHECKBOX;

            // enabled по умолчанию = true
            if (node["enabled"] && node["enabled"].IsScalar()) {
                item.checked = node["enabled"].as<bool>();
            } else {
                item.checked = true;
            }

            if (node["commands"]) {
                std::vector<std::string> cmds;
                const YAML::Node &commandsNode = node["commands"];
                if (commandsNode.IsSequence()) {
                    for (const auto &cmd: commandsNode) {
                        if (cmd && cmd.IsScalar()) {
                            cmds.push_back(cmd.as<std::string>());
                        }
                    }
                } else if (commandsNode.IsScalar()) {
                    cmds.push_back(commandsNode.as<std::string>());
                }

                if (!cmds.empty()) {
                    item.action = std::make_shared<ShAction>(cmds);
                } else {
                    item.action = std::make_shared<YayAction>(item.label);
                }
            } else {
                item.action = std::make_shared<YayAction>(item.label);
            }
        }
    }

    return item;
}


std::vector<MenuItem> parse_section(const YAML::Node &section) {
    std::vector<MenuItem> items;

    if (!section || !section.IsMap()) {
        return items;
    }

    for (auto it = section.begin(); it != section.end(); ++it) {
        if (!it->first.IsScalar()) {
            continue;
        }
        std::string section_name = it->first.as<std::string>();
        MenuItem section_item;
        section_item.label = section_name;
        section_item.type = ITEM_SECTION;

        const YAML::Node &body = it->second;

        const YAML::Node &subsectionsNode = body["sections"];
        const YAML::Node &itemsNode = body["items"];
        if (subsectionsNode) {
            if (subsectionsNode.IsSequence()) {
                for (const auto &subsec: subsectionsNode) {
                    auto children = parse_section(subsec);
                    section_item.children.insert(
                        section_item.children.end(),
                        children.begin(), children.end()
                    );
                }
            }
        }

        if (itemsNode) {
            if (itemsNode.IsSequence()) {
                for (const auto &item: itemsNode) {
                    section_item.children.push_back(parse_item(item));
                }
            } else if (itemsNode.IsScalar()) {
                section_item.children.push_back(parse_item(itemsNode));
            }
        }

        items.push_back(section_item);
    }

    return items;
}

std::vector<MenuItem> parse_root(const YAML::Node &root) {
    std::vector<MenuItem> result;
    if (!root || !root.IsMap()) {
        return result;
    }
    if (root["sections"]) {
        const YAML::Node &sectionsNode = root["sections"];
        if (sectionsNode.IsMap()) {
            auto items = parse_section(sectionsNode);
            result.insert(result.end(), items.begin(), items.end());
        } else if (sectionsNode.IsSequence()) {
            for (const auto &subsec: sectionsNode) {
                auto items = parse_section(subsec);
                result.insert(result.end(), items.begin(), items.end());
            }
        }
    }
    return result;
}

std::vector<std::string> parse_after(const YAML::Node &root) {
    std::vector<std::string> result;
    if (!root || !root.IsMap()) {
        return result;
    }
    if (auto commands = root["after"]["commands"]) {
        if (commands.IsSequence()) {
            for (const auto &cmd: commands) {
                if (cmd && cmd.IsScalar()) {
                    result.push_back(cmd.as<std::string>());
                }
            }
        } else if (commands.IsScalar()) {
            result.push_back(commands.as<std::string>());
        }
    }
    return result;
}

// ----------- Navigation -----------

struct MenuState {
    std::vector<MenuItem> *items;
    int selected = 0;
};

static void cleanup(void) { endwin(); }

static void handle_exit(int sig) {
    (void) sig;
    cleanup();
    std::exit(0);
}

static void handle_resize(int sig) {
    (void) sig;
    endwin();
    refresh();
    clear();
}

void draw_menu(MenuState &state) {
    clear();
    for (size_t i = 0; i < state.items->size(); i++) {
        auto &item = state.items->at(i);
        if ((int) i == state.selected)
            attron(A_REVERSE);

        if (item.type == ITEM_CHECKBOX) {
            mvprintw(i, 0, "[%c] %s", item.checked ? 'x' : ' ', item.label.c_str());
        } else {
            mvprintw(i, 0, "-> %s", item.label.c_str());
        }

        if ((int) i == state.selected)
            attroff(A_REVERSE);
    }
    mvprintw(LINES - 1, 0, "↑/↓ move  →/Enter select  ←/ESC back  q quit");
    refresh();
}

void run_menu(std::vector<MenuItem> &items) {
    MenuState state{&items, 0};

    while (true) {
        draw_menu(state);

        switch (getch()) {
            case KEY_UP:
                if (state.selected > 0) state.selected--;
                break;
            case KEY_DOWN:
                if (state.selected < (int) state.items->size() - 1) state.selected++;
                break;
            case 10: // Enter
            case KEY_RIGHT: {
                MenuItem &sel = state.items->at(state.selected);
                if (sel.type == ITEM_CHECKBOX) {
                    sel.checked = !sel.checked;
                } else if (sel.type == ITEM_SECTION && !sel.children.empty()) {
                    run_menu(sel.children);
                }
                break;
            }
            case 27: // ESC
            case KEY_LEFT:
            case 'q':
            case 'Q':
            case KEY_BACKSPACE:
            case 127:
            case 8:
                return;
        }
    }
}

// ----------- Action collection -----------

void collect_actions(const std::vector<MenuItem> &items, std::vector<std::string> &out) {
    for (auto &item: items) {
        if (item.type == ITEM_CHECKBOX && item.checked && item.action) {
            out.push_back(item.action->render());
        } else if (item.type == ITEM_SECTION) {
            collect_actions(item.children, out);
        }
    }
}

// ----------- Main -----------

std::string write_script_to_file(std::vector<std::string> commands) {
    auto now = std::chrono::system_clock::now();

    std::string filename = "generated-script_" + std::format("{:%d_%m_%Y_%H%M%OS}", now) + ".sh";
    std::ofstream out(filename);

    out << "# Generated script\n";
    for (auto &cmd: commands) {
        out << cmd << "\n";
    }
    out.close();
    return filename;
}

// helper to write response to string
static size_t write_cb(void *contents, size_t size, size_t nmemb, void *userp) {
    ((std::string *) userp)->append((char *) contents, size * nmemb);
    return size * nmemb;
}

std::string download_url(const std::string &url) {
    CURL *curl = curl_easy_init();
    if (!curl) throw std::runtime_error("Failed to init curl");

    std::string response;
    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);

    CURLcode res = curl_easy_perform(curl);
    if (res != CURLE_OK) {
        curl_easy_cleanup(curl);
        throw std::runtime_error(std::string("curl error: ") + curl_easy_strerror(res));
    }
    curl_easy_cleanup(curl);
    return response;
}

int main(int argc, char **argv) {
    setlocale(LC_ALL, "");

    cxxopts::Options options(argv[0], "Arch-based GNU/Linux post install tool");
    options.add_options()
            ("f,file", "YAML config file", cxxopts::value<std::string>())
            ("e,exec", "Execute generated script", cxxopts::value<bool>()->default_value("false"))
            ("w,write", "Write script (optionally takes filename)", cxxopts::value<bool>()->default_value("false"))
            ("h,help", "Print usage");

    auto result = options.parse(argc, argv);

    std::string filename;
    bool exec_flag = false;
    bool write_flag = result["write"].as<bool>();
    std::string out_filename;

    if (result.count("help")) {
        std::cout << options.help() << std::endl;
        return 0;
    }

    if (result.count("file")) {
        filename = result["file"].as<std::string>();
    }
    exec_flag = result["exec"].as<bool>();

    if (write_flag) {
        // find where "-w" occurred
        for (int i = 1; i < argc; i++) {
            if (std::string(argv[i]) == "-w" || std::string(argv[i]) == "--write") {
                // check if there is a following arg and it doesn’t look like an option
                if (i + 1 < argc && argv[i + 1][0] != '-') {
                    out_filename = argv[i + 1];
                }
                break;
            }
        }
    }

    YAML::Node root;
    try {
        if (!filename.empty()) {
            if (
                filename.rfind("http://", 0) == 0
                || filename.rfind("https://", 0) == 0
                || filename.rfind("file://", 0) == 0
            ) {
                std::string yaml_text = download_url(filename);
                root = YAML::Load(yaml_text);
            } else {
                root = YAML::LoadFile(filename);
            }
        } else {
            std::cerr << "Must provide YAML via stdin or -f <file|url>\n";
            return 1;
        }
    } catch (const YAML::Exception &e) {
        std::cerr << "Failed to load YAML: " << e.what() << "\n";
        return 1;
    }


    std::vector<MenuItem> menu = parse_root(root);
    std::vector<std::string> after_commands = parse_after(root);

    initscr();
    noecho();
    keypad(stdscr, TRUE);
    set_escdelay(25);

    atexit(cleanup);
    signal(SIGINT, handle_exit);
    signal(SIGTERM, handle_exit);
    signal(SIGWINCH, handle_resize);

    run_menu(menu);
    cleanup();

    // Collect results
    std::vector<std::string> commands;
    collect_actions(menu, commands);
    commands.insert(commands.end(), after_commands.begin(), after_commands.end());

    // Handle write
    if (write_flag) {
        std::string final_name = out_filename.empty() ? write_script_to_file(commands) : out_filename;

        if (!out_filename.empty()) {
            std::ofstream out(final_name);
            out << "# Generated script\n";
            for (auto &cmd: commands) out << cmd << "\n";
        }

        std::cout << "# Script saved to ./" << final_name << std::endl;

        // Handle execution if requested
        if (exec_flag) {
            std::cout << "Executing script...\n";
            std::string exec_cmd = "bash " + final_name;
            int ret = std::system(exec_cmd.c_str());
            std::cout << "Execution finished with code " << ret << std::endl;
        }
    } else if (exec_flag) {
        // Execute without writing
        std::cout << "Executing directly...\n";
        for (auto &cmd: commands) {
            int ret = std::system(cmd.c_str());
            if (ret != 0) {
                std::cerr << "Command failed: " << cmd << "\n";
                break;
            }
        }
    } else {
        std::cout << "# Generated script\n";
        for (auto &cmd: commands) std::cout << cmd << std::endl;
    }

    return 0;
}
