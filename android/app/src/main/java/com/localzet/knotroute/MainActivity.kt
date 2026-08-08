package com.localzet.knotroute

import android.Manifest
import android.app.*
import android.content.*
import android.content.pm.PackageManager
import android.graphics.Color
import android.graphics.Typeface
import android.net.Uri
import android.net.VpnService
import android.os.*
import android.provider.Settings
import android.view.*
import android.widget.*
import com.google.android.material.bottomnavigation.BottomNavigationView
import com.google.android.material.button.MaterialButton
import com.google.android.material.card.MaterialCardView
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

class MainActivity : Activity() {
    private lateinit var root: LinearLayout
    private lateinit var content: FrameLayout
    private lateinit var statusText: TextView
    private lateinit var bottom: BottomNavigationView
    private val handler = Handler(Looper.getMainLooper())
    private var page = PAGE_HOME
    private var pendingCa: ByteArray? = null
    private var waitGeneration = 0

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 40)
        handleJoinIntent(intent)
        startForegroundService(Intent(this, KnotService::class.java))
        buildShell()
        waitGeneration++
        waitForCore(0, waitGeneration, null)
        handler.post(statusTick)
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent); setIntent(intent)
        if (intent != null && handleJoinIntent(intent)) { restartCore(); showPage(PAGE_NETWORK) }
    }

    private fun buildShell() {
        window.statusBarColor = Color.rgb(13,17,23); window.navigationBarColor = Color.rgb(13,17,23)
        root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setBackgroundColor(Color.rgb(13,17,23)) }
        root.setOnApplyWindowInsetsListener { v, insets ->
            val top: Int; val bottomInset: Int
            if (Build.VERSION.SDK_INT >= 30) { val bars = insets.getInsets(WindowInsets.Type.systemBars()); top = bars.top; bottomInset = bars.bottom }
            else { @Suppress("DEPRECATION") top = insets.systemWindowInsetTop; @Suppress("DEPRECATION") bottomInset = insets.systemWindowInsetBottom }
            v.setPadding(0, top, 0, bottomInset); insets
        }
        val header = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL; setPadding(dp(20),dp(14),dp(20),dp(12))
            addView(LinearLayout(this@MainActivity).apply { orientation = LinearLayout.VERTICAL
                addView(TextView(this@MainActivity).apply { text="KnotRoute"; textSize=23f; setTextColor(Color.WHITE); setTypeface(typeface,Typeface.BOLD) })
                addView(TextView(this@MainActivity).apply { text=getString(R.string.tagline); textSize=11f; setTextColor(Color.rgb(137,148,164)) })
            }, LinearLayout.LayoutParams(0,ViewGroup.LayoutParams.WRAP_CONTENT,1f))
            statusText = TextView(this@MainActivity).apply { text=getString(R.string.starting); textSize=12f; setTextColor(Color.rgb(255,201,112)); setPadding(dp(12),dp(8),dp(12),dp(8)); background=rounded(Color.rgb(37,42,50),18) }
            addView(statusText)
        }
        root.addView(header)
        content = FrameLayout(this).also { root.addView(it, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,0,1f)) }
        bottom = BottomNavigationView(this).apply {
            setBackgroundColor(Color.rgb(17,22,29)); itemIconTintList = null
            menu.add(0,PAGE_HOME,0,getString(R.string.home)).setIcon(R.drawable.ic_home)
            menu.add(0,PAGE_CHATS,1,getString(R.string.chats)).setIcon(R.drawable.ic_chat)
            menu.add(0,PAGE_CATALOG,2,getString(R.string.catalog)).setIcon(R.drawable.ic_catalog)
            menu.add(0,PAGE_NETWORK,3,getString(R.string.network)).setIcon(R.drawable.ic_network)
            menu.add(0,PAGE_MORE,4,getString(R.string.more)).setIcon(R.drawable.ic_more)
            setOnItemSelectedListener { showPage(it.itemId); true }
        }
        root.addView(bottom)
        setContentView(root); root.requestApplyInsets(); showPage(PAGE_HOME)
    }

    private fun showPage(which: Int) { page=which; content.removeAllViews(); content.addView(when(which){PAGE_CHATS->chatsPage();PAGE_CATALOG->catalogPage();PAGE_NETWORK->networkPage();PAGE_MORE->morePage();else->homePage()}) }

    private fun pageScroll(builder: LinearLayout.()->Unit)=ScrollView(this).apply { addView(LinearLayout(this@MainActivity).apply { orientation=LinearLayout.VERTICAL;setPadding(dp(18),dp(8),dp(18),dp(28));builder() }) }
    private fun heading(title:String,subtitle:String)=LinearLayout(this).apply { orientation=LinearLayout.VERTICAL;setPadding(0,dp(4),0,dp(16));addView(TextView(this@MainActivity).apply{text=title;textSize=26f;setTypeface(typeface,Typeface.BOLD);setTextColor(Color.WHITE)});addView(TextView(this@MainActivity).apply{text=subtitle;textSize=13f;setTextColor(Color.rgb(145,156,171));setPadding(0,dp(5),0,0)}) }
    private fun card(title:String,value:String,detail:String,action:String?=null,onClick:(()->Unit)?=null)=MaterialCardView(this).apply { radius=dp(18).toFloat();cardElevation=0f;setCardBackgroundColor(Color.rgb(22,28,36));strokeWidth=1;strokeColor=Color.rgb(38,47,58);layoutParams=LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,ViewGroup.LayoutParams.WRAP_CONTENT).apply{bottomMargin=dp(12)};addView(LinearLayout(this@MainActivity).apply{orientation=LinearLayout.VERTICAL;setPadding(dp(18),dp(16),dp(18),dp(16));addView(TextView(this@MainActivity).apply{text=title.uppercase();textSize=10f;letterSpacing=.12f;setTextColor(Color.rgb(135,148,166))});addView(TextView(this@MainActivity).apply{text=value;textSize=19f;setTypeface(typeface,Typeface.BOLD);setTextColor(Color.WHITE);setPadding(0,dp(5),0,0)});if(detail.isNotBlank())addView(TextView(this@MainActivity).apply{text=detail;textSize=13f;setTextColor(Color.rgb(162,173,187));setPadding(0,dp(6),0,0)});if(action!=null&&onClick!=null)addView(button(action,onClick).apply{layoutParams=LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,dp(46)).apply{topMargin=dp(12)}})}) }
    private fun button(text:String,onClick:()->Unit)=MaterialButton(this).apply { this.text=text;isAllCaps=false;cornerRadius=dp(14);setOnClickListener{onClick()} }
    private fun input(label:String,value:String="",multi:Boolean=false):Pair<TextInputLayout,TextInputEditText>{val edit=TextInputEditText(this).apply{setText(value);if(multi){minLines=3;gravity=Gravity.TOP}else setSingleLine()};val layout=TextInputLayout(this).apply{hint=label;boxBackgroundMode=TextInputLayout.BOX_BACKGROUND_OUTLINE;setBoxCornerRadii(dp(14).toFloat(),dp(14).toFloat(),dp(14).toFloat(),dp(14).toFloat());addView(edit);layoutParams=LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,ViewGroup.LayoutParams.WRAP_CONTENT).apply{bottomMargin=dp(10)}};return layout to edit}

    private fun homePage()=pageScroll {
        addView(heading(getString(R.string.home),getString(R.string.home_subtitle)))
        val status=statusObject();val peers=status?.optJSONArray("peers")?.length()?:0;val routes=status?.optJSONArray("routes")?.length()?:0
        addView(card(getString(R.string.connection),if(KnotRuntime.ready()) getString(R.string.connected) else getString(R.string.starting),getString(R.string.connection_detail,peers,routes)))
        val profile=runCatching{JSONObject(KnotRuntime.userProfileJson())}.getOrNull();addView(card(getString(R.string.profile),profile?.optString("display_name")?:"—",short(profile?.optString("id")?:KnotRuntime.userId())))
        val social=runCatching{JSONObject(KnotRuntime.socialStateJson())}.getOrNull();val posts=social?.optJSONArray("posts")
        addView(TextView(this@MainActivity).apply{text=getString(R.string.feed);textSize=20f;setTypeface(typeface,Typeface.BOLD);setTextColor(Color.WHITE);setPadding(0,dp(8),0,dp(10))})
        if(posts==null||posts.length()==0)addView(card(getString(R.string.feed_empty),getString(R.string.feed_first_post),getString(R.string.feed_alpha_note),getString(R.string.new_post)){postDialog()})
        else { for(i in 0 until minOf(posts.length(),3)){val post=posts.optJSONObject(i);val author=post?.optJSONObject("author");addView(card(author?.optString("display_name")?.ifBlank{getString(R.string.profile)}?:getString(R.string.profile),post?.optString("text")?:"",getString(R.string.signed_post)))};addView(button(getString(R.string.new_post)){postDialog()});addView(button(getString(R.string.refresh_feed)){refreshFeeds()}) }
        addView(card(getString(R.string.browser_integration),getString(R.string.browser_open_knot),getString(R.string.browser_beta_note),getString(R.string.enable)){requestBrowserVpn()})
        addView(card(getString(R.string.ca_title),getString(R.string.ca_local_root),getString(R.string.ca_android_hint),getString(R.string.export_ca)){exportCA()})
    }

    private fun chatsPage()=pageScroll {
        addView(heading(getString(R.string.chats),getString(R.string.chats_subtitle)))
        val state=runCatching{JSONObject(KnotRuntime.socialStateJson())}.getOrNull();val contacts=state?.optJSONObject("contacts")
        if(contacts==null||contacts.length()==0)addView(card(getString(R.string.no_contacts),getString(R.string.add_first_contact),getString(R.string.contact_explain)))
        else contacts.keys().forEach{uid->val c=contacts.optJSONObject(uid);val p=c?.optJSONObject("profile");addView(card(p?.optString("display_name")?:short(uid),short(uid),c?.optString("node")?:"",getString(R.string.message)){messageDialog(uid,p?.optString("display_name")?:uid)})}
        val node=input(getString(R.string.contact_node));val alias=input(getString(R.string.contact_name));addView(node.first);addView(alias.first);addView(button(getString(R.string.add_contact)){background { KnotRuntime.addContact(node.second.text.toString(),alias.second.text.toString()); runOnUiThread{showPage(PAGE_CHATS)} }})
    }


    private fun postDialog(){val edit=EditText(this).apply{hint=getString(R.string.post_hint);minLines=3};MaterialAlertDialogBuilder(this).setTitle(getString(R.string.new_post)).setView(edit).setNegativeButton(android.R.string.cancel,null).setPositiveButton(getString(R.string.publish)){_,_->background{KnotRuntime.createPost(edit.text.toString(),"");runOnUiThread{showPage(PAGE_HOME)}}}.show()}
    private fun refreshFeeds(){background{val state=runCatching{JSONObject(KnotRuntime.socialStateJson())}.getOrNull();val contacts=state?.optJSONObject("contacts");if(contacts!=null){contacts.keys().forEach{uid->runCatching{KnotRuntime.fetchContactFeed(uid)}}};runOnUiThread{showPage(PAGE_HOME);toast(getString(R.string.feed_refreshed))}}}

    private fun messageDialog(uid:String,name:String){val edit=EditText(this).apply{hint=getString(R.string.message);minLines=2};MaterialAlertDialogBuilder(this).setTitle(name).setView(edit).setNegativeButton(android.R.string.cancel,null).setPositiveButton(getString(R.string.send)){_,_->background{KnotRuntime.sendMessage(uid,edit.text.toString())}}.show()}

    private fun catalogPage()=pageScroll {
        addView(heading(getString(R.string.catalog),getString(R.string.catalog_subtitle)))
        val status=statusObject();var count=0
        status?.optJSONArray("known_services")?.let{arr->for(i in 0 until arr.length()){val s=arr.optJSONObject(i);val domain=s.optString("domain");if(domain.isNotBlank()){val meta=s.optJSONObject("metadata");val title=meta?.optString("title")?.takeIf{it.isNotBlank()}?:getString(R.string.published_service);addView(card(title,domain,meta?.optString("description")?:"",getString(R.string.open_browser)){openBrowser(domain)});count++}}}
        status?.optJSONArray("routes")?.let{arr->for(i in 0 until arr.length()){val r=arr.optJSONObject(i);val node=r.optString("domain");val services=r.optJSONArray("services")?:continue;for(j in 0 until services.length()){val svc=services.optString(j);if(svc=="kr-chat")continue;val domain="$svc.$node";addView(card(svc,domain,getString(R.string.node_service),getString(R.string.open_browser)){openBrowser(domain)});count++}}}
        if(count==0)addView(card(getString(R.string.catalog_empty),getString(R.string.nothing_found),getString(R.string.catalog_privacy)))
    }

    private fun networkPage()=pageScroll {
        addView(heading(getString(R.string.network),getString(R.string.network_subtitle)))
        val profiles=NetworkProfiles.all(this@MainActivity);var selected=NetworkProfiles.activeIndex(this@MainActivity).coerceIn(0,profiles.lastIndex);val p=profiles[selected]
        val name=input(getString(R.string.profile_name),p.name);val network=input(getString(R.string.network_id),p.networkId);val beacons=input(getString(R.string.beacon_urls),p.beacons.joinToString("\n"),true);addView(name.first);addView(network.first);addView(beacons.first)
        val advanced=LinearLayout(this@MainActivity).apply{orientation=LinearLayout.VERTICAL;visibility=View.GONE};val hops=input(getString(R.string.circuit_hops),p.circuitHops.toString());advanced.addView(hops.first);addView(button(getString(R.string.advanced_settings)){advanced.visibility=if(advanced.visibility==View.VISIBLE)View.GONE else View.VISIBLE});addView(advanced)
        addView(button(getString(R.string.save_restart)){val n=network.second.text.toString().trim();if(n.isNotEmpty()&&!n.startsWith("kn_")){toast(getString(R.string.invalid_network_id));return@button};val bs=beacons.second.text.toString().lines().map{it.trim()}.filter{it.isNotEmpty()};val invalid=bs.firstOrNull{validateBeaconUrl(it)!=null};if(invalid!=null){toast(validateBeaconUrl(invalid)!!);return@button};profiles[selected]=NetworkProfile(name.second.text.toString().ifBlank{getString(R.string.default_profile)},n,bs,hops.second.text.toString().toIntOrNull()?.coerceIn(1,8)?:3);NetworkProfiles.save(this@MainActivity,profiles,selected);restartCore()})
    }

    private fun morePage()=pageScroll {
        addView(heading(getString(R.string.more),getString(R.string.more_subtitle)))
        val profile=runCatching{JSONObject(KnotRuntime.userProfileJson())}.getOrNull();val name=input(getString(R.string.profile_name),profile?.optString("display_name")?:"");val bio=input(getString(R.string.bio),profile?.optString("bio")?:"",true);addView(name.first);addView(bio.first);addView(button(getString(R.string.save_profile)){background{KnotRuntime.setUserProfile(name.second.text.toString(),bio.second.text.toString());runOnUiThread{toast(getString(R.string.saved))}}})
        addView(card(getString(R.string.user_id),short(KnotRuntime.userId()),getString(R.string.user_id_explain)))
        addView(card(getString(R.string.node_address),short(KnotRuntime.nodeAddress()),getString(R.string.node_id_explain)))
        addView(card(getString(R.string.ca_title),getString(R.string.ca_manage),getString(R.string.ca_android_hint),getString(R.string.configure_ca)){caProfileDialog()})
        addView(button(getString(R.string.export_ca)){exportCA()})
        addView(button(getString(R.string.open_security_settings)){runCatching{startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS))}})
    }


    private fun caProfileDialog(){
        if(!KnotRuntime.ready()){toast(getString(R.string.core_starting));return}
        val ca=runCatching{JSONObject(KnotRuntime.caProfileJson())}.getOrNull()?:run{toast(getString(R.string.failed));return}
        val subject=ca.optJSONObject("subject")?:JSONObject()
        fun csv(key:String):String{val a=subject.optJSONArray(key)?:return "";return (0 until a.length()).joinToString(", "){a.optString(it)}}
        val box=LinearLayout(this).apply{orientation=LinearLayout.VERTICAL;setPadding(dp(8),dp(8),dp(8),dp(8))}
        val cn=input("Common Name (CN)",subject.optString("common_name"));box.addView(cn.first)
        val org=input("Organization (O)",csv("organization"));box.addView(org.first)
        val ou=input("Organizational Unit (OU)",csv("organizational_unit"));box.addView(ou.first)
        val country=input("Country (C)",csv("country"));box.addView(country.first)
        val province=input("State / Province (ST)",csv("province"));box.addView(province.first)
        val locality=input("Locality (L)",csv("locality"));box.addView(locality.first)
        val street=input(getString(R.string.ca_street),csv("street_address"));box.addView(street.first)
        val postal=input(getString(R.string.ca_postal),csv("postal_code"));box.addView(postal.first)
        val validity=input(getString(R.string.ca_validity),ca.optInt("validity_days",3650).toString());box.addView(validity.first)
        val scroll=ScrollView(this).apply{addView(box)}
        fun saveProfile(rotate:Boolean){
            val days=validity.second.text.toString().toIntOrNull()?:run{toast(getString(R.string.ca_invalid_validity));return}
            background{
                KnotRuntime.setCaProfile(cn.second.text.toString(),org.second.text.toString(),ou.second.text.toString(),country.second.text.toString(),province.second.text.toString(),locality.second.text.toString(),street.second.text.toString(),postal.second.text.toString(),days)
                if(rotate){KnotRuntime.rotateCa()}
                runOnUiThread{toast(if(rotate)getString(R.string.ca_rotated_export_again) else getString(R.string.ca_profile_saved))}
            }
        }
        MaterialAlertDialogBuilder(this).setTitle(getString(R.string.ca_profile_title)).setMessage(getString(R.string.ca_profile_explain)).setView(scroll).setNegativeButton(android.R.string.cancel,null).setPositiveButton(getString(R.string.save)){_,_->saveProfile(false)}.setNeutralButton(getString(R.string.rotate_ca)){_,_->saveProfile(true)}.show()
    }

    private fun requestBrowserVpn(){if(Build.VERSION.SDK_INT<Build.VERSION_CODES.Q){toast(getString(R.string.browser_requires_android10));return};if(!KnotRuntime.ready()){toast(getString(R.string.core_starting));return};val intent=VpnService.prepare(this);if(intent!=null)startActivityForResult(intent,REQ_VPN)else startBrowserVpn()}
    private fun startBrowserVpn(){startForegroundService(Intent(this,KnotBrowserVpnService::class.java).setAction(KnotBrowserVpnService.ACTION_START));toast(getString(R.string.browser_integration_started))}
    private fun exportCA(){if(!KnotRuntime.ready()){toast(getString(R.string.core_starting));return};try{val cert=CertificateFactory.getInstance("X.509").generateCertificate(ByteArrayInputStream(KnotRuntime.rootCaPem().toByteArray())) as X509Certificate;pendingCa=cert.encoded;val i=Intent(Intent.ACTION_CREATE_DOCUMENT).apply{addCategory(Intent.CATEGORY_OPENABLE);type="application/x-x509-ca-cert";putExtra(Intent.EXTRA_TITLE,"knotroute-root-ca.crt")};startActivityForResult(i,REQ_CA)}catch(e:Throwable){toast(e.message?:getString(R.string.ca_failed))}}

    override fun onActivityResult(requestCode:Int,resultCode:Int,data:Intent?){super.onActivityResult(requestCode,resultCode,data);if(requestCode==REQ_VPN&&resultCode==RESULT_OK){startBrowserVpn();return};if(requestCode==REQ_CA&&resultCode==RESULT_OK&&data?.data!=null){try{contentResolver.openOutputStream(data.data!!)!!.use{it.write(pendingCa?:ByteArray(0))};pendingCa=null;MaterialAlertDialogBuilder(this).setTitle(getString(R.string.ca_saved_title)).setMessage(getString(R.string.ca_install_steps)).setNegativeButton(android.R.string.ok,null).setPositiveButton(getString(R.string.open_settings)){_,_->runCatching{startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS))}}.show()}catch(e:Throwable){toast(e.message?:getString(R.string.ca_failed))}}}

    private fun handleJoinIntent(intent:Intent):Boolean{val uri=intent.data?:return false;val profile=NetworkProfiles.importJoinUri(this,uri)?:return false;val profiles=NetworkProfiles.all(this);val existing=profiles.indexOfFirst{it.networkId==profile.networkId};val index=if(existing>=0){profiles[existing]=profile;existing}else{profiles.add(profile);profiles.lastIndex};NetworkProfiles.save(this,profiles,index);return true}
    private fun waitForCore(attempt:Int,generation:Int,after:Long?){if(generation!=waitGeneration||isFinishing)return;val fresh=after==null||KnotRuntime.generation()>after;if(KnotRuntime.ready()&&fresh){statusText.text=getString(R.string.connected);statusText.setTextColor(Color.rgb(140,255,193));showPage(page);return};if(attempt>200){statusText.text=getString(R.string.failed);statusText.setTextColor(Color.rgb(255,112,126));KnotRuntime.lastError?.let{toast(it)};return};handler.postDelayed({waitForCore(attempt+1,generation,after)},200)}
    private fun restartCore(){statusText.text=getString(R.string.starting);val previous=KnotRuntime.generation();waitGeneration++;startForegroundService(Intent(this,KnotService::class.java).setAction(KnotService.ACTION_RESTART));waitForCore(0,waitGeneration,previous)}
    private val statusTick=object:Runnable{override fun run(){if(!isFinishing){statusText.text=if(KnotRuntime.ready())getString(R.string.connected)else getString(R.string.starting);handler.postDelayed(this,2500)}}}
    private fun statusObject():JSONObject?=runCatching{JSONObject(KnotRuntime.statusJson())}.getOrNull()
    private fun openBrowser(domain:String){runCatching{startActivity(Intent(Intent.ACTION_VIEW,Uri.parse("https://$domain/")))}.onFailure{toast(it.message?:getString(R.string.failed))}}
    private fun background(block:()->Unit)=Thread{try{block()}catch(t:Throwable){runOnUiThread{toast(t.message?:t.javaClass.simpleName)}}}.start()
    private fun validateBeaconUrl(raw:String):String?=try{val u=Uri.parse(raw);if((u.scheme!="https"&&u.scheme!="http")||u.host.isNullOrBlank())getString(R.string.invalid_beacon_url)else if(u.port==7447)getString(R.string.beacon_relay_port_error)else if(!u.path.isNullOrBlank()&&u.path!="/")getString(R.string.beacon_root_error)else null}catch(_:Throwable){getString(R.string.invalid_beacon_url)}
    private fun toast(s:String)=Toast.makeText(this,s,Toast.LENGTH_LONG).show()
    private fun rounded(color:Int,radius:Int)=android.graphics.drawable.GradientDrawable().apply{setColor(color);cornerRadius=dp(radius).toFloat()}
    private fun short(s:String):String=if(s.length<=38)s else s.take(18)+"…"+s.takeLast(14)
    private fun dp(v:Int)=(v*resources.displayMetrics.density).toInt()
    override fun onDestroy(){handler.removeCallbacks(statusTick);super.onDestroy()}

    companion object { private const val PAGE_HOME=1;private const val PAGE_CHATS=2;private const val PAGE_CATALOG=3;private const val PAGE_NETWORK=4;private const val PAGE_MORE=5;private const val REQ_VPN=60;private const val REQ_CA=61 }
}
