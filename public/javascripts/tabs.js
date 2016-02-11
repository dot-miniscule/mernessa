/* ====================== MAINPAGE  ===================== */
/* THIS IS THE JAVASCRIPT FOR THE MAIN PAGE */

// Logs out of current user
function logout() {
  document.cookie = 'user_name=Guest';
  window.location.reload();
}

// Returns true if the user is not valid
function validUser(username) {
  return username !== 'Guest';
}

// Verifies the user, and submits the Post form
function submitForm(username, type, updateTime) {
  console.log('checking user');
  if (!validUser(username)) {
    showModal('login');
    return false;
  }
  return checkDB(type, updateTime);
}

// Function to create the login modal, to save on having to write the html markup multiple
// times. Creates the content dynamically, to be hidden and shown.
function showModal(type) {
  var modal = $('#loginModal');
  var modalHTML =
    '<div class="modal-dialog">'+
      '<div class="modal-content">'+
        '<div class="modal-header">'+
          '<button type="button" class="close" data-dismiss="modal">&times;</button>'+
          '<h4 class="modal-title">Login fail</h4>'+
        '</div><!--/.modal-header-->'+
        '<div class="modal-body">';

  if (type === 'login') {
    modalHTML +=
          '<p>Must be logged in to continue.</p>'+
          '<p><a href="/login">Login now.</a></p>';
  } else if (type === 'joinSO') {
    modalHTML +=
          '<p>Must be registered on StackOverflow to continue.</p>'+
          '<p><a href="https://stackoverflow.com/users/join" target="_blank">Join now.</a></p>';
  }
  modalHTML +=
        '</div>'+
        '<div class="modal-footer">'+
           '<button type="button" class="btn btn-default" data-dismiss="modal">Close</button>'+
        '</div>'+
      '</div>'+
    '</div>';
  modal.html(modalHTML);
  modal.modal('show');
}

// Checks the update time of the database.
// If the current page is outdated, check if the questions to be updated have been recently changed.
// If they have, send alert for confirmation.
// Else submit the form.
function checkDB(buttonPressed, updateTime) {
  if(buttonPressed == 'reopen') {
    buttonPressed = 'pending';
  }

  // Send Post request to get if the database has been updated, and which questions have been updated.
  $.post('/dbUpdated?time='+updateTime, function( dataJSON ) {

    // Get the title of the question being updated
    var title;
    if (buttonPressed == 'submit') { // If the form was updated through the menu

      // The title is the text of the menu's grandparent's cousin.
      title = $('.new_state_menu option[value!="no_change"]:selected').parent().parent().parent().siblings('.question').children('.question_title').text();
    } else if (buttonPressed != '') { // If the form was updated through the buttons

      // The title is the text of the button's grandparent's cousin.
      title = $('.one-click.clicked').parent().parent().parent().siblings('.question').children('.question_title').text();

      // Replace the menu's value with the value of the button.
      $('.new_state_menu').val('no_change');
      $('.one-click.clicked').parent().siblings('.new_state_menu').val(buttonPressed);
    }

    // Parse the data recieved in the Post request
    var data = JSON.parse(dataJSON);

    if (data.Updated) { // If the database has been updated.
      // Writing the text to display in the confirm dialog box
      confirm_text = 'Database has been updated\nChanges:\n';
      for (i = 0; i < data.Questions.length; i++) {
        if (data.Questions[i] === title) {
          confirm_text += '\t* ' + title + '\n';
          break;
        }
      }
      
      // Text to write in the confirmation dialog box.
      confirm_text += '\nDo you wish to continue submit?';
      if (confirm_text.indexOf('*') > -1) { // If the question being updated is one of the recently changed questions.
        // Send confirmation of submit.
        if (!confirm(confirm_text)) {
          $('form select').val('no_change');
          return;
        }
      }
    }

    // Submit the form as a post request
    $('#stateForm').submit(function() {
      // Set cookie for webui to check.
      document.cookie = 'submitting=true';

      // Get the current state and the question id of the changed question.
      var qnElems = $('.new_state_menu option[value!="no_change"]:selected').parent().attr('name').split("_");
      var cache = qnElems[0];
      var qnID = qnElems[1];

      // Get the new state of the changed question.
      var newState = $('.new_state_menu option[value!="no_change"]:selected').val();

      // Post the state with the current state and id of the question.
      // When done posting, redirect back to the original page.
      $.post( '/', {'cache': cache, 'question_id': qnID, 'state': newState})
        .done(function( data ) {
          window.location = window.location.href.split('#')[0];
        });
      return false;
    });
    $('#stateForm').submit();
  });
}

// Highlight the selected navigation link


// Nav bar active state
$(".navigation a").on("click", function(){
  // Setting the active tab
  $(".nav.nav-tabs").find(".active").removeClass(".active");
  $(this).parent().addClass(".active");

  // Adjusting the url to match active tab
  var tab = $(this).attr("href").substring(1);
  var subpage = window.location.href.slice(window.location.href.lastIndexOf('/'));
  var url = removeQuery('tab', subpage)
  url = addQuery('tab', tab, url);
  window.history.replaceState('', document.title, url);

  setActiveTab(window.location.href);
});

// Function to remove query, i.e if questions are filtered by tag, user, URL, keyword etc.
// URL splits on any query (?) and the & to maintain the currently selected tab.
function removePageQuery(queryString) {
  console.log(queryString);
  var sourceURL = window.location.href;
  var queries = sourceURL.split(queryString);
  var tab = sourceURL.split('&')[1];
  console.log(queries[0]+'?'+tab);
  window.location = queries[0]+'?'+tab;
}


// removes the currently active tab, and then finds the new one based on the URL
// resizes the container div height
function setActiveTab(sourceURL) {
  $('.active').removeClass('active');
  var queries = sourceURL.slice(window.location.href.indexOf('?')+1).split('&');
  var param;
  for (var i = 0; i < queries.length; i++) {
    var urlparam = queries[i].split('=');
    if (urlparam[0] == 'tab') {
      param = '#' + urlparam[1];
      break;
    }
  }
  $('li a[href="'+param+'"]').parent().addClass('active');
  $(param).addClass('active');
}

// Function to remove any content from the question table
// If the user has previewed a question, and either cancelled its submission
// or if they have previewed a question and want to preview a new one the table needs to be
// cleared so content doesn't stack on top of itself
function clearTextPreserveChildren(element) {
  element.contents().filter(function() {
    return(this.nodeType == 3);
  }).remove();
}


// When a user enters a URL or ID into the search box on the find question page
// Checks that the input from the form is either a URL or an id number
// Parses the incoming string to isolate the question ID from the URL
// If its not valid, it yells at you, and instructs how to format 
function pullQuestionFromStackOverflow() {
  var alert = $('#new-question-alert');
  alert.hide();
  var query = $('input#searchTerm').val();

  if(!$.isNumeric(query)) {
    var check1 = canSplit(query, "http://stackoverflow.com/questions/");

    if(check1) {
      var queryA = query.split("http://stackoverflow.com/questions/");
      var check2 = canSplit(query.split("http://stackoverflow.com/questions/")[1], "/");

      if(check2) {
        queryB = query.split("http://stackoverflow.com/questions/")[1].split("/")[0];

        if($.isNumeric(queryB)) {
          pullNewQn(queryB);   
        }
      } else {
        removeAlertClass(alert);
        alert.html('Not a valid search term.<br>Please enter a valid StackOverflow URL' +
          'StackOverflow question ID. <br><br> Eg,'+
          ' http://stackoverflow.com/questions/123456/example-question-title or 123456');
        alert.addClass('alert-warning');
        alert.show();
      }
    }
  } else {
    pullNewQn(query);
  }
}

// Removes any of the following from an alert object:
// success, info, warning, danger.
// This is to ensure that new alerts assigned to the same object have the correct
// level and there are no conflicts. Eg, on the addQuestion page, if a success alert
// is followed by a danger, because a question could not be found or the search
// query they entered was not valid, this will ensure the alert level is removed 
// before a new one is added. 
function removeAlertClass(alert) {
  var classList = alert.attr("class").toString().split(' ');
  for(var i=0; i < classList.length; i++) {
    if(classList[i] === 'alert-success' || classList[i] === 'alert-info' || 
      classList[i] === 'alert-warning' || classList[i] === 'alert-danger') {
      alert.removeClass(classList[i]);
    }
  }
}

// Formats a post request to the server to pull the question from Stack Overflow
// Once a response is received, the relevant elements are cleared to be refilled with 
// fresh data. If the response is empty or undefined, an error is displayed to the user.
// Otherwise, a display function is called to read the data into the page.
function pullNewQn(query) {
  $.post('/pullNewQn?id='+query, function( data ) {
    var table = $('table');
    var alert = $('#new-question-alert');
    clearTextPreserveChildren(table);
    clearTextPreserveChildren(alert);
    if(data == undefined || data == "") {
      alert.hide();
      removeAlertClass(alert);
      alert.addClass('alert-warning');
      alert.html("Question was unable to be pulled from the database. Sorry.")
      alert.show();
    } else {
    displayNewQuestion(data);
    }
  });
}


// Parses a JSON object and displays it in the table for viewing
// First, it determines if the data is a new or existing question
// new/existing require different button functionality and different naming
// A previously existing question has an extra field called Message, which 
// basically says that this is an existing question.
// This means this question must have some sort of state assigned to it
// It needs to display this state, and the appropriate buttons for usage
// The select menu is only shown on questions marked as unanswered, pending or
// updating. The answered state only has a single button marked Reopen.
// 
// The JSON is saved in local storage, so that if that question is submitted 
// into the database the question information is still available
function displayNewQuestion(data) {
  var question = JSON.parse(data);
  var alert = $('#new-question-alert');
  var type = question.Message;
  var btn = $('.function-button');
  var menu = $('.new_state_menu');
  var options = {}
  var cancel = $('.cancel-button');
  $('.questionExists').empty();
  menu.empty();
  menu.off('change');
  btn.off('click');
  cancel.addClass('hidden');
  menu.append($("<option disabled selected></option>").text('Choose an option...'))
  btn.attr('name', 'unanswered_'+ question.Question_id);
  btn.click().addClass('clicked');
  btn.attr('value', 'Pending');
  clearTextPreserveChildren($('.questionOwner'));

  if(type == undefined) {
    menu.removeClass('hidden');
    cancel.removeClass('hidden');
    options = {
      "unanswered":"Unanswered", 
      "answered":"Answered",
      "updating":"Updating"
    };
    btn.off('click');
    btn.on('click', function() {
      addQuestionToStackTracker(data, btn.attr('value').toLowerCase());
    });
    cancel.on('click', function() {
      clearNewQuestionTable();
    })
    menu.on('change', function() { 
      addQuestionToStackTracker(data, menu.val());
    });
  } else { 
    type = question.State;
    if(type == "pending") {
      menu.removeClass('hidden');
      options = {
        "updating":"Updating", 
        "answered":""
      }
      btn.attr('value', 'Answered');
    } else if(type == "updating") {
      menu.removeClass('hidden');
      options = {
        "pending":"Pending",
        "answered":""
      };
      btn.attr('value', 'Answered');
    } else if(type == "answered") {
      menu.addClass('hidden');
      options = {
        "pending":""
      }
      btn.attr('value', 'Reopen');
    } else {
      menu.removeClass('hidden');
      options = { 
        "answered":"Answered",
        "updating":"Updating",
        "pending":""
      };
      btn.attr('value', 'Pending');
    }

    btn.attr('name', type + '_' + question.Question_id);
    menu.attr('name', type + '_' + question.Question_id);
    btn.off('click');
    btn.on('click', function() { 
      submitForm(localStorage["currentUser"], btn.prop('value').toLowerCase(), 
      localStorage["lastUpdateTime"]);
    });
    menu.off('change');
    menu.on('change', function() {
      submitForm(localStorage["currentUser"], menu.prop('value').toLowerCase(),
        localStorage["lastUpdateTime"])
    });
  }

  $.each(options, function(value, key) {
      menu.append($("<option></option>").attr("value", value).text(key));
  });
  menu.children().last().hide();
  if(question.Message != undefined && question.Message != "") {
    alert.hide();
    removeAlertClass(alert);
    alert.html(
      question.Message);
    alert.addClass('alert-info');
    alert.show();
  } else {
    alert.hide();
    removeAlertClass(alert);
    alert.html(
      'New question with ID: '+question.Question_id+' found!');
    alert.addClass('alert-info');
    alert.show();
  }
  $('a.question_title').attr("href", question.Link).children('h4').html(question.Title);
  $('table td.question .bodySnippet').html(question.Body);
  $('ul.tags').empty();
  $.each(question.Tags, function(i, item) {
    $('.tagContainer ul.tags').append('<a href="/tag?tagSearch='+item
      +'"><li class="tag">'+item+'</li></a>');
  });

  if(question.UserDisplayName != undefined && question.UserDisplayName != "") {
    $('.questionOwner').html('Question marked as '+question.State
      +' by <a href=\"/user?id='+question.UserID+'\">'+question.UserDisplayName
      +'</a> on '+ question.Time);
  }
  $('table').removeClass('hidden');
}


// If the user chooses to cancel their question rather than add it to the database
// This function will clear the table of any content, as well as remove anything
// in local storage.
// If you cancel it gives you attitude.
function clearNewQuestionTable() {
  var table = $('table');
  clearTextPreserveChildren(table);
  table.addClass('hidden');
  var alert = $('#new-question-alert');
  clearTextPreserveChildren(alert);
  alert.hide();
  alert.html('Oh. Okay :(');
  removeAlertClass(alert);
  alert.addClass('alert-warning');
  alert.show();
  $('.questionExists').empty();
  $('.questionExists').html("That's fine. We didn't want that question anyway.")
}


// Function to save aspects of the reply in local storage to post updated question state
// to the backend when the user selects an action.
function saveState(user, lastUpdateTime) {
  localStorage["currentUser"] = user;
  localStorage["lastUpdateTime"] = lastUpdateTime;
}


// Helper function to ensure input validation on the add question field
// If the user enters something that cannot be recognised, it should return false
 function canSplit(str, token) {
  return(str || '').split(token).length > 1;
 }

// Function to post to server to add the new question to StackTrackers database
// newQuestion is the stringified JSON data that is cached in localStorage
// Checks if the user is logged in, alerts if not
// If they are, it completes the post request and 
function addQuestionToStackTracker(newQuestion, newState) {
  if (!validUser(localStorage["currentUser"])) {
    buildModal();
    $('#loginModal').modal('show');
  } else {
    var data = {"question": newQuestion, "state": newState};
    $.ajax({
      type: "POST",
      url: "/addNewQuestion",
      processData: false,
      contentType: 'application/json',
      data: JSON.stringify({"Question":newQuestion, "State":newState}),
      success: function( data ) {
        var alert = $('#new-question-alert');
        removeAlertClass(alert);
        alert.addClass('alert-success');
        var alertString
         if(newState == "unanswered") {
            alertString = "Question successfully added to list of"+newState+" questions!"
          } else {
            alertString = "Question successfully added to "+localStorage["currentUser"]+
            "\'s "+newState+" questions!"
          }
        alert.html(alertString);
        alert.show();
      }
    });
  }
}

//-------- SETTING PREP WHEN DOCUMENT LOADS -------//
$(document).ready(function() {
  // Set neccessary cookies for user verification.
  setCookies();

  // Remove code query from url.
  // If code query not in url, url remains the same.
  var url = removeQuery('code', window.location.href);
  window.history.replaceState("", document.title, url);

  // Remove any queries without a value attached.
  clearEmptyQueries();

  var subpage = window.location.href.split('?')[0].slice(window.location.href.lastIndexOf('/'));
  // Add tab query to pages with questions.
  if (window.location.search.indexOf('tab') == -1 &&
    subpage.indexOf('viewTags') == -1 && subpage.indexOf('viewUsers') == -1 &&
    subpage.indexOf('addQuestion') == -1 && subpage.indexOf('user') == -1) {
      var addedPath = subpage + addQuery('tab', 'unanswered', window.location.search);
      window.history.replaceState('', document.title, addedPath);
  } else if (window.location.search.indexOf('page') == -1 && (subpage.indexOf('viewTags') != -1 ||
    subpage.indexOf('viewUsers') != -1)) { // Add page number query to viewing pages.
      var addedPath = subpage + addQuery('page', '1', window.location.search);
      window.history.replaceState('', document.title, addedPath);
  } else if(window.location.search.indexOf('tab') == -1 &&
    subpage.indexOf('user') != -1) {
      var addedPath = subpage + addQuery('tab', 'pending', window.location.search);
      window.history.replaceState('', document.title, addedPath);
  }

  setActiveTab(window.location.href);
});

//-------- COOKIES --------//
function setCookies() {
  // Set user cookie
  document.cookie = 'user_name=' + localStorage['currentUser'];
}

//-------- Removing queries ---------//
function clearEmptyQueries() {
  // Get the host url
  var urlParts = window.location.href.split('?');
  var newURL = urlParts[0].slice(window.location.href.lastIndexOf('/'));

  var param;
  var params_arr = [];
  var queryString = urlParts[1];

  // Get the list of queries.
  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');

    // Range through queries, splitting them into the query and the value.
    // If the value is empty, remove the query.
    for (var i = params_arr.length -1; i >= 0; i--) {
      query = params_arr[i].split('=')[1];
      if (query == '') {
        params_arr.splice(i, 1);
      }
    }
    newURL += '?' + params_arr.join('&');
  }

  // Replace the current URL with the updated URL
  window.history.replaceState('', document.title, newURL);
}

// Returns path including modified query assuming sourceURL does not
// contain the key
function addQuery(key, query, sourceURL) {
  var newURL = sourceURL;
  if (sourceURL.indexOf('?') !== -1) {
    if (sourceURL.split('?')[1] !== '') {
      newURL += '&';
    }
  } else {
    newURL += '?';
  }
  newURL += key + '=' + query;
  return newURL;
}

// Returns path without query specified
function removeQuery(key, sourceURL) {
  var newURL = sourceURL.split('?')[0];
  var param;
  var params_arr = [];
  var queryString = sourceURL.split('?')[1];

  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');
    for (var i = params_arr.length -1; i >= 0; i--) {
      param = params_arr[i].split('=')[0];
      if (param == key) {
        params_arr.splice(i, 1);
      }
    }
    if (params_arr.length > 0) {
      newURL += '?' + params_arr.join('&');
    }
  }
  return newURL;
}
